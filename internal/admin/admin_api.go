package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"

	"github.com/RunzhiZhao/long-gate/internal/config"
	"github.com/RunzhiZhao/long-gate/internal/etcdv3"
	"github.com/RunzhiZhao/long-gate/internal/router"
)

// AdminAPI 管理 API 服务器
type AdminAPI struct {
	etcdClient *clientv3.Client
	router     *router.Router
	logger     *zap.Logger
	mux        *http.ServeMux
}

// NewAdminAPI 创建管理 API
func NewAdminAPI(etcdClient *clientv3.Client, r *router.Router, logger *zap.Logger) *AdminAPI {
	api := &AdminAPI{
		etcdClient: etcdClient,
		router:     r,
		logger:     logger,
		mux:        http.NewServeMux(),
	}
	api.setupRoutes()
	return api
}

// setupRoutes 设置路由
func (api *AdminAPI) setupRoutes() {
	// 路由管理
	api.mux.HandleFunc("/admin/routes", api.handleRoutes)
	api.mux.HandleFunc("/admin/routes/", api.handleRouteByID)

	// 上游管理
	api.mux.HandleFunc("/admin/upstreams", api.handleUpstreams)
	api.mux.HandleFunc("/admin/upstreams/", api.handleUpstreamByID)

	// 健康检查
	api.mux.HandleFunc("/admin/health", api.handleHealth)
}

// ServeHTTP 实现 http.Handler
func (api *AdminAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	api.mux.ServeHTTP(w, r)
}

// --- 路由管理 API ---

// handleRoutes 处理路由列表
func (api *AdminAPI) handleRoutes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		api.listRoutes(w, r)
	case http.MethodPost:
		api.createRoute(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleRouteByID 处理单个路由
func (api *AdminAPI) handleRouteByID(w http.ResponseWriter, r *http.Request) {
	// 提取路由 ID (简化实现)
	routeID := r.URL.Path[len("/admin/routes/"):]
	if routeID == "" {
		http.Error(w, "Route ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		api.getRoute(w, r, routeID)
	case http.MethodPut:
		api.updateRoute(w, r, routeID)
	case http.MethodDelete:
		api.deleteRoute(w, r, routeID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// listRoutes 获取路由列表
func (api *AdminAPI) listRoutes(w http.ResponseWriter, r *http.Request) {
	routes := api.router.ListRoutes()
	api.respondJSON(w, http.StatusOK, map[string]interface{}{
		"total": len(routes),
		"data":  routes,
	})
}

// getRoute 获取单个路由
func (api *AdminAPI) getRoute(w http.ResponseWriter, r *http.Request, routeID string) {
	route := api.router.GetRoute(routeID)
	if route == nil {
		http.Error(w, "Route not found", http.StatusNotFound)
		return
	}
	api.respondJSON(w, http.StatusOK, route)
}

// createRoute 创建路由
func (api *AdminAPI) createRoute(w http.ResponseWriter, r *http.Request) {
	var route config.Route
	if err := json.NewDecoder(r.Body).Decode(&route); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// 设置元数据
	route.CreateTime = time.Now().Unix()
	route.UpdateTime = time.Now().Unix()
	route.Version = 1

	// 验证
	if err := route.Validate(); err != nil {
		http.Error(w, fmt.Sprintf("Validation failed: %v", err), http.StatusBadRequest)
		return
	}

	// 保存到 ETCD
	data, _ := route.ToJSON()
	key := etcdv3.RoutePrefix + route.ID

	ctx, cancel := r.Context(), func() {}
	defer cancel()

	if _, err := api.etcdClient.Put(ctx, key, string(data)); err != nil {
		api.logger.Error("failed to save route to etcd", zap.Error(err))
		http.Error(w, "Failed to save route", http.StatusInternalServerError)
		return
	}

	api.respondJSON(w, http.StatusCreated, route)
}

// updateRoute 更新路由
func (api *AdminAPI) updateRoute(w http.ResponseWriter, r *http.Request, routeID string) {
	var route config.Route
	if err := json.NewDecoder(r.Body).Decode(&route); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	route.ID = routeID
	route.UpdateTime = time.Now().Unix()

	if err := route.Validate(); err != nil {
		http.Error(w, fmt.Sprintf("Validation failed: %v", err), http.StatusBadRequest)
		return
	}

	// 更新到 ETCD
	data, _ := route.ToJSON()
	key := etcdv3.RoutePrefix + route.ID

	ctx, cancel := r.Context(), func() {}
	defer cancel()

	if _, err := api.etcdClient.Put(ctx, key, string(data)); err != nil {
		api.logger.Error("failed to update route in etcd", zap.Error(err))
		http.Error(w, "Failed to update route", http.StatusInternalServerError)
		return
	}

	api.respondJSON(w, http.StatusOK, route)
}

// deleteRoute 删除路由
func (api *AdminAPI) deleteRoute(w http.ResponseWriter, r *http.Request, routeID string) {
	key := etcdv3.RoutePrefix + routeID

	ctx, cancel := r.Context(), func() {}
	defer cancel()

	if _, err := api.etcdClient.Delete(ctx, key); err != nil {
		api.logger.Error("failed to delete route from etcd", zap.Error(err))
		http.Error(w, "Failed to delete route", http.StatusInternalServerError)
		return
	}

	api.respondJSON(w, http.StatusOK, map[string]string{
		"message": "Route deleted successfully",
	})
}

// --- 上游管理 API ---

// handleUpstreams 处理上游列表
func (api *AdminAPI) handleUpstreams(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		api.listUpstreams(w, r)
	case http.MethodPost:
		api.createUpstream(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleUpstreamByID 处理单个上游
func (api *AdminAPI) handleUpstreamByID(w http.ResponseWriter, r *http.Request) {
	upstreamID := r.URL.Path[len("/admin/upstreams/"):]
	if upstreamID == "" {
		http.Error(w, "Upstream ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		api.getUpstream(w, r, upstreamID)
	case http.MethodPut:
		api.updateUpstream(w, r, upstreamID)
	case http.MethodDelete:
		api.deleteUpstream(w, r, upstreamID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// listUpstreams 获取上游列表
func (api *AdminAPI) listUpstreams(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := r.Context(), func() {}
	defer cancel()

	resp, err := api.etcdClient.Get(ctx, etcdv3.UpstreamPrefix, clientv3.WithPrefix())
	if err != nil {
		http.Error(w, "Failed to fetch upstreams", http.StatusInternalServerError)
		return
	}

	upstreams := make([]*config.Upstream, 0)
	for _, kv := range resp.Kvs {
		var u config.Upstream
		if err := json.Unmarshal(kv.Value, &u); err != nil {
			continue
		}
		upstreams = append(upstreams, &u)
	}

	api.respondJSON(w, http.StatusOK, map[string]interface{}{
		"total": len(upstreams),
		"data":  upstreams,
	})
}

// getUpstream 获取单个上游
func (api *AdminAPI) getUpstream(w http.ResponseWriter, r *http.Request, upstreamID string) {
	ctx, cancel := r.Context(), func() {}
	defer cancel()

	key := etcdv3.UpstreamPrefix + upstreamID
	resp, err := api.etcdClient.Get(ctx, key)
	if err != nil || len(resp.Kvs) == 0 {
		http.Error(w, "Upstream not found", http.StatusNotFound)
		return
	}

	var upstream config.Upstream
	if err := json.Unmarshal(resp.Kvs[0].Value, &upstream); err != nil {
		http.Error(w, "Failed to parse upstream", http.StatusInternalServerError)
		return
	}

	api.respondJSON(w, http.StatusOK, upstream)
}

// createUpstream 创建上游
func (api *AdminAPI) createUpstream(w http.ResponseWriter, r *http.Request) {
	var upstream config.Upstream
	if err := json.NewDecoder(r.Body).Decode(&upstream); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	upstream.CreateTime = time.Now().Unix()
	upstream.UpdateTime = time.Now().Unix()
	upstream.Version = 1

	if err := upstream.Validate(); err != nil {
		http.Error(w, fmt.Sprintf("Validation failed: %v", err), http.StatusBadRequest)
		return
	}

	data, _ := upstream.ToJSON()
	key := etcdv3.UpstreamPrefix + upstream.ID

	ctx, cancel := r.Context(), func() {}
	defer cancel()

	if _, err := api.etcdClient.Put(ctx, key, string(data)); err != nil {
		http.Error(w, "Failed to save upstream", http.StatusInternalServerError)
		return
	}

	api.respondJSON(w, http.StatusCreated, upstream)
}

// updateUpstream 更新上游
func (api *AdminAPI) updateUpstream(w http.ResponseWriter, r *http.Request, upstreamID string) {
	var upstream config.Upstream
	if err := json.NewDecoder(r.Body).Decode(&upstream); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	upstream.ID = upstreamID
	upstream.UpdateTime = time.Now().Unix()

	if err := upstream.Validate(); err != nil {
		http.Error(w, fmt.Sprintf("Validation failed: %v", err), http.StatusBadRequest)
		return
	}

	data, _ := upstream.ToJSON()
	key := etcdv3.UpstreamPrefix + upstream.ID

	ctx, cancel := r.Context(), func() {}
	defer cancel()

	if _, err := api.etcdClient.Put(ctx, key, string(data)); err != nil {
		http.Error(w, "Failed to update upstream", http.StatusInternalServerError)
		return
	}

	api.respondJSON(w, http.StatusOK, upstream)
}

// deleteUpstream 删除上游
func (api *AdminAPI) deleteUpstream(w http.ResponseWriter, r *http.Request, upstreamID string) {
	key := etcdv3.UpstreamPrefix + upstreamID

	ctx, cancel := r.Context(), func() {}
	defer cancel()

	if _, err := api.etcdClient.Delete(ctx, key); err != nil {
		http.Error(w, "Failed to delete upstream", http.StatusInternalServerError)
		return
	}

	api.respondJSON(w, http.StatusOK, map[string]string{
		"message": "Upstream deleted successfully",
	})
}

// --- 健康检查 ---

// handleHealth 健康检查端点
func (api *AdminAPI) handleHealth(w http.ResponseWriter, r *http.Request) {
	api.respondJSON(w, http.StatusOK, map[string]string{
		"status": "healthy",
	})
}

// respondJSON 响应 JSON
func (api *AdminAPI) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
