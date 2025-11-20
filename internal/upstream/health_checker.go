package upstream

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/RunzhiZhao/long-gate/internal/config"
)

// HealthChecker 健康检查器
type HealthChecker struct {
	upstreams map[string]*config.Upstream
	logger    *zap.Logger
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.RWMutex
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(logger *zap.Logger) *HealthChecker {
	ctx, cancel := context.WithCancel(context.Background())
	return &HealthChecker{
		upstreams: make(map[string]*config.Upstream),
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start 启动健康检查
func (hc *HealthChecker) Start() {
	go hc.runHealthCheckLoop()
	hc.logger.Info("health checker started")
}

// Stop 停止健康检查
func (hc *HealthChecker) Stop() {
	hc.cancel()
	hc.logger.Info("health checker stopped")
}

// AddUpstream 添加上游服务到健康检查
func (hc *HealthChecker) AddUpstream(upstream *config.Upstream) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.upstreams[upstream.ID] = upstream
}

// RemoveUpstream 移除上游服务
func (hc *HealthChecker) RemoveUpstream(upstreamID string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	delete(hc.upstreams, upstreamID)
}

// runHealthCheckLoop 健康检查循环
func (hc *HealthChecker) runHealthCheckLoop() {
	ticker := time.NewTicker(5 * time.Second) // 默认 5 秒检查一次
	defer ticker.Stop()

	for {
		select {
		case <-hc.ctx.Done():
			return
		case <-ticker.C:
			hc.checkAllUpstreams()
		}
	}
}

// checkAllUpstreams 检查所有上游服务
func (hc *HealthChecker) checkAllUpstreams() {
	hc.mu.RLock()
	upstreams := make([]*config.Upstream, 0, len(hc.upstreams))
	for _, u := range hc.upstreams {
		upstreams = append(upstreams, u)
	}
	hc.mu.RUnlock()

	// 并发检查
	var wg sync.WaitGroup
	for _, upstream := range upstreams {
		if upstream.HealthCheck == nil || !upstream.HealthCheck.Enabled {
			continue
		}

		wg.Add(1)
		go func(u *config.Upstream) {
			defer wg.Done()
			hc.checkUpstream(u)
		}(upstream)
	}
	wg.Wait()
}

// checkUpstream 检查单个上游服务
func (hc *HealthChecker) checkUpstream(upstream *config.Upstream) {
	for _, target := range upstream.Targets {
		// 跳过已知不健康的节点（避免频繁检查）
		if time.Since(target.LastCheckAt) < time.Duration(upstream.HealthCheck.Interval)*time.Second {
			continue
		}

		healthy := hc.checkTarget(upstream, target)
		hc.updateTargetStatus(upstream, target, healthy)
	}
}

// checkTarget 检查单个目标节点
func (hc *HealthChecker) checkTarget(upstream *config.Upstream, target *config.Target) bool {
	ctx, cancel := context.WithTimeout(hc.ctx, time.Duration(upstream.HealthCheck.Timeout)*time.Second)
	defer cancel()

	switch upstream.HealthCheck.Type {
	case "http", "":
		return hc.checkHTTP(ctx, upstream, target)
	case "tcp":
		return hc.checkTCP(ctx, target)
	default:
		hc.logger.Warn("unsupported health check type",
			zap.String("type", upstream.HealthCheck.Type),
			zap.String("upstream", upstream.ID))
		return false
	}
}

// checkHTTP HTTP 健康检查
func (hc *HealthChecker) checkHTTP(ctx context.Context, upstream *config.Upstream, target *config.Target) bool {
	url := fmt.Sprintf("http://%s%s", target.Address, upstream.HealthCheck.Path)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false
	}

	client := &http.Client{
		Timeout: time.Duration(upstream.HealthCheck.Timeout) * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		hc.logger.Debug("health check failed",
			zap.String("upstream", upstream.ID),
			zap.String("target", target.Address),
			zap.Error(err))
		return false
	}
	defer resp.Body.Close()

	// 2xx 或 3xx 状态码认为健康
	return resp.StatusCode >= 200 && resp.StatusCode < 400
}

// checkTCP TCP 健康检查（简化实现）
func (hc *HealthChecker) checkTCP(ctx context.Context, target *config.Target) bool {
	// TODO: 实现 TCP 连接检查
	return true
}

// updateTargetStatus 更新目标节点状态
func (hc *HealthChecker) updateTargetStatus(upstream *config.Upstream, target *config.Target, healthy bool) {
	target.LastCheckAt = time.Now()

	if healthy {
		target.FailCount = 0
		if target.Status != config.TargetStatusHealthy {
			// 需要连续成功 N 次才标记为健康
			if target.FailCount <= -upstream.HealthCheck.HealthyThreshold {
				upstream.UpdateTargetStatus(target.Address, config.TargetStatusHealthy)
				hc.logger.Info("target became healthy",
					zap.String("upstream", upstream.ID),
					zap.String("target", target.Address))
			} else {
				target.FailCount--
			}
		}
	} else {
		target.FailCount++
		target.LastFailAt = time.Now()

		if target.Status != config.TargetStatusUnhealthy {
			// 连续失败 N 次后标记为不健康
			if target.FailCount >= upstream.HealthCheck.UnhealthyThreshold {
				upstream.UpdateTargetStatus(target.Address, config.TargetStatusUnhealthy)
				hc.logger.Warn("target became unhealthy",
					zap.String("upstream", upstream.ID),
					zap.String("target", target.Address),
					zap.Int("fail_count", target.FailCount))
			}
		}
	}
}
