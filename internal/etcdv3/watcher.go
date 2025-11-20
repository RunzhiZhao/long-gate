package etcdv3

import (
	"context"
	"fmt"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"

	"github.com/RunzhiZhao/long-gate/internal/config"
	"github.com/RunzhiZhao/long-gate/internal/router"
)

const (
	// ETCD Key 前缀
	RoutePrefix    = "/gateway/routes/"
	UpstreamPrefix = "/gateway/upstreams/"
)

// ConfigWatcher 配置监听器
type ConfigWatcher struct {
	client    *clientv3.Client
	router    *router.Router
	upstreams map[string]*config.Upstream // upstream_id -> Upstream
	logger    *zap.Logger
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewConfigWatcher 创建配置监听器
func NewConfigWatcher(client *clientv3.Client, r *router.Router, logger *zap.Logger) *ConfigWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &ConfigWatcher{
		client:    client,
		router:    r,
		upstreams: make(map[string]*config.Upstream),
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start 启动监听
func (w *ConfigWatcher) Start() error {
	// 1. 首次加载全量配置
	if err := w.loadAllConfigs(); err != nil {
		return fmt.Errorf("failed to load initial configs: %w", err)
	}

	// 2. 启动 Watch 协程
	go w.watchRoutes()
	go w.watchUpstreams()

	w.logger.Info("config watcher started")
	return nil
}

// Stop 停止监听
func (w *ConfigWatcher) Stop() {
	w.cancel()
	w.logger.Info("config watcher stopped")
}

// loadAllConfigs 加载全量配置
func (w *ConfigWatcher) loadAllConfigs() error {
	ctx, cancel := context.WithTimeout(w.ctx, 10*time.Second)
	defer cancel()

	// 加载路由
	routes, err := w.loadRoutes(ctx)
	if err != nil {
		return fmt.Errorf("failed to load routes: %w", err)
	}
	if err := w.router.LoadRoutes(routes); err != nil {
		return fmt.Errorf("failed to initialize router: %w", err)
	}

	// 加载上游
	upstreams, err := w.loadUpstreams(ctx)
	if err != nil {
		return fmt.Errorf("failed to load upstreams: %w", err)
	}
	for _, upstream := range upstreams {
		w.upstreams[upstream.ID] = upstream
	}

	w.logger.Info("loaded initial configs",
		zap.Int("routes", len(routes)),
		zap.Int("upstreams", len(upstreams)))
	return nil
}

// loadRoutes 从 ETCD 加载所有路由
func (w *ConfigWatcher) loadRoutes(ctx context.Context) ([]*config.Route, error) {
	resp, err := w.client.Get(ctx, RoutePrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	routes := make([]*config.Route, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		route := &config.Route{}
		if err := route.FromJSON(kv.Value); err != nil {
			w.logger.Error("failed to parse route",
				zap.String("key", string(kv.Key)),
				zap.Error(err))
			continue
		}
		routes = append(routes, route)
	}
	return routes, nil
}

// loadUpstreams 从 ETCD 加载所有上游
func (w *ConfigWatcher) loadUpstreams(ctx context.Context) ([]*config.Upstream, error) {
	resp, err := w.client.Get(ctx, UpstreamPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	upstreams := make([]*config.Upstream, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		upstream := &config.Upstream{}
		if err := upstream.FromJSON(kv.Value); err != nil {
			w.logger.Error("failed to parse upstream",
				zap.String("key", string(kv.Key)),
				zap.Error(err))
			continue
		}
		upstreams = append(upstreams, upstream)
	}
	return upstreams, nil
}

// watchRoutes 监听路由变化
func (w *ConfigWatcher) watchRoutes() {
	watchChan := w.client.Watch(w.ctx, RoutePrefix, clientv3.WithPrefix())

	for {
		select {
		case <-w.ctx.Done():
			return
		case watchResp := <-watchChan:
			if watchResp.Err() != nil {
				w.logger.Error("watch routes error", zap.Error(watchResp.Err()))
				// 重连逻辑
				time.Sleep(5 * time.Second)
				watchChan = w.client.Watch(w.ctx, RoutePrefix, clientv3.WithPrefix())
				continue
			}

			for _, event := range watchResp.Events {
				w.handleRouteEvent(event)
			}
		}
	}
}

// watchUpstreams 监听上游变化
func (w *ConfigWatcher) watchUpstreams() {
	watchChan := w.client.Watch(w.ctx, UpstreamPrefix, clientv3.WithPrefix())

	for {
		select {
		case <-w.ctx.Done():
			return
		case watchResp := <-watchChan:
			if watchResp.Err() != nil {
				w.logger.Error("watch upstreams error", zap.Error(watchResp.Err()))
				time.Sleep(5 * time.Second)
				watchChan = w.client.Watch(w.ctx, UpstreamPrefix, clientv3.WithPrefix())
				continue
			}

			for _, event := range watchResp.Events {
				w.handleUpstreamEvent(event)
			}
		}
	}
}

// handleRouteEvent 处理路由事件
func (w *ConfigWatcher) handleRouteEvent(event *clientv3.Event) {
	routeID := extractID(string(event.Kv.Key), RoutePrefix)

	switch event.Type {
	case clientv3.EventTypePut:
		route := &config.Route{}
		if err := route.FromJSON(event.Kv.Value); err != nil {
			w.logger.Error("failed to parse route from watch event",
				zap.String("key", string(event.Kv.Key)),
				zap.Error(err))
			return
		}

		if err := w.router.AddRoute(route); err != nil {
			w.logger.Error("failed to add/update route",
				zap.String("route_id", routeID),
				zap.Error(err))
			return
		}

		w.logger.Info("route updated", zap.String("route_id", routeID))

	case clientv3.EventTypeDelete:
		if err := w.router.DeleteRoute(routeID); err != nil {
			w.logger.Error("failed to delete route",
				zap.String("route_id", routeID),
				zap.Error(err))
			return
		}

		w.logger.Info("route deleted", zap.String("route_id", routeID))
	}
}

// handleUpstreamEvent 处理上游事件
func (w *ConfigWatcher) handleUpstreamEvent(event *clientv3.Event) {
	upstreamID := extractID(string(event.Kv.Key), UpstreamPrefix)

	switch event.Type {
	case clientv3.EventTypePut:
		upstream := &config.Upstream{}
		if err := upstream.FromJSON(event.Kv.Value); err != nil {
			w.logger.Error("failed to parse upstream from watch event",
				zap.String("key", string(event.Kv.Key)),
				zap.Error(err))
			return
		}

		w.upstreams[upstreamID] = upstream
		w.logger.Info("upstream updated", zap.String("upstream_id", upstreamID))

	case clientv3.EventTypeDelete:
		delete(w.upstreams, upstreamID)
		w.logger.Info("upstream deleted", zap.String("upstream_id", upstreamID))
	}
}

// GetUpstream 获取上游服务
func (w *ConfigWatcher) GetUpstream(id string) (*config.Upstream, bool) {
	upstream, ok := w.upstreams[id]
	return upstream, ok
}

// extractID 从 ETCD Key 中提取 ID
// 例: /gateway/routes/route-123 -> route-123
func extractID(key, prefix string) string {
	return strings.TrimPrefix(key, prefix)
}
