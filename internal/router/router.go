package router

import (
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/RunzhiZhao/long-gate/internal/config"
)

// Router 路由引擎
type Router struct {
	routes atomic.Value // *RouteTable，支持原子更新
	mu     sync.RWMutex
}

// RouteTable 路由表（不可变结构）
type RouteTable struct {
	routes   []*config.Route
	indexMap map[string]*config.Route // id -> route 快速查找
}

// NewRouter 创建路由引擎
func NewRouter() *Router {
	r := &Router{}
	r.routes.Store(&RouteTable{
		routes:   make([]*config.Route, 0),
		indexMap: make(map[string]*config.Route),
	})
	return r
}

// LoadRoutes 加载路由表（全量替换）
func (r *Router) LoadRoutes(routes []*config.Route) error {
	// 验证并排序路由（按优先级降序）
	validRoutes := make([]*config.Route, 0, len(routes))
	for _, route := range routes {
		if err := route.Validate(); err != nil {
			continue // 跳过无效路由
		}
		validRoutes = append(validRoutes, route)
	}

	// 按优先级排序（优先级高的在前）
	sort.Slice(validRoutes, func(i, j int) bool {
		return validRoutes[i].Priority > validRoutes[j].Priority
	})

	// 构建新的路由表
	newTable := &RouteTable{
		routes:   validRoutes,
		indexMap: make(map[string]*config.Route),
	}
	for _, route := range validRoutes {
		newTable.indexMap[route.ID] = route
	}

	// 原子替换
	r.routes.Store(newTable)
	return nil
}

// AddRoute 添加单个路由（增量更新）
func (r *Router) AddRoute(route *config.Route) error {
	if err := route.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	oldTable := r.routes.Load().(*RouteTable)

	// 创建新路由列表
	newRoutes := make([]*config.Route, 0, len(oldTable.routes)+1)
	replaced := false

	for _, existing := range oldTable.routes {
		if existing.ID == route.ID {
			newRoutes = append(newRoutes, route) // 替换
			replaced = true
		} else {
			newRoutes = append(newRoutes, existing)
		}
	}

	if !replaced {
		newRoutes = append(newRoutes, route) // 新增
	}

	// 重新排序
	sort.Slice(newRoutes, func(i, j int) bool {
		return newRoutes[i].Priority > newRoutes[j].Priority
	})

	// 构建新表
	newTable := &RouteTable{
		routes:   newRoutes,
		indexMap: make(map[string]*config.Route),
	}
	for _, r := range newRoutes {
		newTable.indexMap[r.ID] = r
	}

	r.routes.Store(newTable)
	return nil
}

// DeleteRoute 删除路由
func (r *Router) DeleteRoute(routeID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	oldTable := r.routes.Load().(*RouteTable)

	newRoutes := make([]*config.Route, 0, len(oldTable.routes))
	for _, route := range oldTable.routes {
		if route.ID != routeID {
			newRoutes = append(newRoutes, route)
		}
	}

	newTable := &RouteTable{
		routes:   newRoutes,
		indexMap: make(map[string]*config.Route),
	}
	for _, r := range newRoutes {
		newTable.indexMap[r.ID] = r
	}

	r.routes.Store(newTable)
	return nil
}

// Match 匹配路由
func (r *Router) Match(req *http.Request) (*config.Route, map[string]string) {
	table := r.routes.Load().(*RouteTable)

	path := req.URL.Path
	method := req.Method
	host := req.Host
	headers := extractHeaders(req)

	// 按优先级顺序匹配
	for _, route := range table.routes {
		if route.Match(path, method, host, headers) {
			// 提取路径参数（如果是参数化路由）
			params := extractPathParams(route.Predicates.Path, path)
			return route, params
		}
	}

	return nil, nil
}

// GetRoute 根据 ID 获取路由
func (r *Router) GetRoute(id string) *config.Route {
	table := r.routes.Load().(*RouteTable)
	return table.indexMap[id]
}

// ListRoutes 获取所有路由
func (r *Router) ListRoutes() []*config.Route {
	table := r.routes.Load().(*RouteTable)
	routes := make([]*config.Route, len(table.routes))
	copy(routes, table.routes)
	return routes
}

// extractHeaders 提取 HTTP 头部
func extractHeaders(req *http.Request) map[string]string {
	headers := make(map[string]string)
	for key, values := range req.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
	return headers
}

// extractPathParams 提取路径参数 (简单实现)
// 如: pattern=/api/users/:id, path=/api/users/123 -> {id: "123"}
func extractPathParams(pattern, path string) map[string]string {
	params := make(map[string]string)

	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")

	if len(patternParts) != len(pathParts) {
		return params
	}

	for i, part := range patternParts {
		if strings.HasPrefix(part, ":") {
			paramName := strings.TrimPrefix(part, ":")
			params[paramName] = pathParts[i]
		}
	}

	return params
}
