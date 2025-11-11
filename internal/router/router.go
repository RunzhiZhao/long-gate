package router

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/RunzhiZhao/long-gate/internal/config"
	"github.com/RunzhiZhao/long-gate/internal/middleware"
	"github.com/RunzhiZhao/long-gate/internal/proxy"
)

// RouteEntry 存储一个路由的所有信息，包括服务和中间件链
type RouteEntry struct {
	Config  config.RouteConfig
	Service config.ServiceConfig
	Proxy   http.Handler // 使用 http.Handler 统一接口
}

// RouterManager 负责管理路由表
type RouterManager struct {
	mu sync.RWMutex
	// 路由表：使用 Path 作为 Key 快速查找
	routes map[string]*RouteEntry
}

var globalRouter *RouterManager

func InitRouter() {
	globalRouter = &RouterManager{
		routes: make(map[string]*RouteEntry),
	}
	// 初始加载路由（尽管 LoadAndWatchConfig 已经做了一次，这里确保初始化）
	globalRouter.updateRoutes()
}

// updateRoutes 从配置中重新加载路由表，实现热更新
func (rm *RouterManager) updateRoutes() {
	newRoutes := make(map[string]*RouteEntry)

	for _, routeCfg := range config.GetRoutes() {
		svcCfg, ok := config.GetServiceConfig(routeCfg.ServiceID)
		if !ok {
			fmt.Printf("Warning: Service ID '%s' not found for route '%s'\n", routeCfg.ServiceID, routeCfg.Path)
			continue
		}

		var p http.Handler
		var err error

		// 根据服务类型选择代理实现
		switch svcCfg.Type {
		case "http":
			p, err = proxy.NewHTTPProxy(svcCfg.Addr)
		// case "rpc":
		// ⚠️ 暂不实现 rpc 代理的 NewRPCProxy，MVP 只支持 http
		// p, err = proxy.NewRPCProxy(svcCfg.Addr)
		default:
			fmt.Printf("Warning: Unknown service type '%s' for service '%s'\n", svcCfg.Type, routeCfg.ServiceID)
			continue
		}

		if err != nil {
			fmt.Printf("Error creating proxy for route '%s': %v\n", routeCfg.Path, err)
			continue
		}

		newRoutes[routeCfg.Path] = &RouteEntry{
			Config:  routeCfg,
			Service: svcCfg,
			Proxy:   p,
		}
	}

	// 原子替换路由表
	rm.mu.Lock()
	rm.routes = newRoutes
	rm.mu.Unlock()

	fmt.Printf("Routes successfully updated. Total active routes: %d\n", len(newRoutes))
}

// HandleRequest 是网关的统一入口处理函数
func HandleRequest(w http.ResponseWriter, r *http.Request) {
	// 路由查找
	routeEntry, ok := globalRouter.getRoute(r.URL.Path)
	if !ok {
		http.Error(w, "Route Not Found", http.StatusNotFound)
		return
	}

	// 创建并执行中间件链
	chain := middleware.NewChain(routeEntry.Config)
	if !chain.Execute(w, r) {
		// 中间件返回 false，请求已被中断和响应（如鉴权失败）
		return
	}

	// 执行反向代理
	routeEntry.Proxy.ServeHTTP(w, r)
}

// getRoute 查找匹配的路由
func (rm *RouterManager) getRoute(path string) (*RouteEntry, bool) {
	// ⚠️ 注意：为了简单 MVP，这里只进行精确匹配。
	// 生产环境应使用高性能路由库（如 go-chi/patricia-trie）进行前缀和模糊匹配。
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	// 每次查找时，尝试更新路由，确保配置中心更改能及时生效
	globalRouter.updateRoutes()

	entry, ok := rm.routes[path]
	return entry, ok
}
