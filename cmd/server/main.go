package main

import (
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RunzhiZhao/long-gate/internal/config"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"

	"github.com/RunzhiZhao/long-gate/internal/admin"
	"github.com/RunzhiZhao/long-gate/internal/balancer"
	"github.com/RunzhiZhao/long-gate/internal/etcdv3"
	"github.com/RunzhiZhao/long-gate/internal/middleware"
	"github.com/RunzhiZhao/long-gate/internal/router"
	"github.com/RunzhiZhao/long-gate/internal/upstream"
)

// Gateway 网关核心
type Gateway struct {
	router        *router.Router
	watcher       *etcdv3.ConfigWatcher
	healthChecker *upstream.HealthChecker
	adminAPI      *admin.AdminAPI
	logger        *zap.Logger

	// 中间件链
	globalChain *middleware.Chain
}

func main() {
	// 初始化日志
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// 连接 ETCD
	etcdClient, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"localhost:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		logger.Fatal("failed to connect to etcd", zap.Error(err))
	}
	defer etcdClient.Close()

	// 创建网关实例
	gateway := NewGateway(etcdClient, logger)

	// 启动服务
	if err := gateway.Start(); err != nil {
		logger.Fatal("failed to start gateway", zap.Error(err))
	}

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down gateway...")
	gateway.Stop()
}

// NewGateway 创建网关实例
func NewGateway(etcdClient *clientv3.Client, logger *zap.Logger) *Gateway {
	// 创建路由引擎
	r := router.NewRouter()

	// 创建配置监听器
	watcher := etcdv3.NewConfigWatcher(etcdClient, r, logger)

	// 创建健康检查器
	healthChecker := upstream.NewHealthChecker(logger)

	// 创建管理 API
	adminAPI := admin.NewAdminAPI(etcdClient, r, logger)

	// 创建全局中间件链
	globalChain := middleware.NewChain(
		middleware.Recovery(logger),
		middleware.Logger(logger),
		middleware.RequestID(),
		middleware.CORS(),
	)

	return &Gateway{
		router:        r,
		watcher:       watcher,
		healthChecker: healthChecker,
		adminAPI:      adminAPI,
		logger:        logger,
		globalChain:   globalChain,
	}
}

// Start 启动网关
func (g *Gateway) Start() error {
	// 1. 启动配置监听
	if err := g.watcher.Start(); err != nil {
		return err
	}

	// 2. 启动健康检查
	g.healthChecker.Start()

	// 3. 启动管理 API (端口 9000)
	go func() {
		g.logger.Info("admin api listening on :9000")
		if err := http.ListenAndServe(":9000", g.adminAPI); err != nil {
			g.logger.Error("admin api error", zap.Error(err))
		}
	}()

	// 4. 启动数据面服务器 (端口 8080)
	go func() {
		g.logger.Info("gateway listening on :8080")
		if err := http.ListenAndServe(":8080", g); err != nil {
			g.logger.Error("gateway error", zap.Error(err))
		}
	}()

	return nil
}

// Stop 停止网关
func (g *Gateway) Stop() {
	g.watcher.Stop()
	g.healthChecker.Stop()
}

// ServeHTTP 处理请求（数据面核心）
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 创建上下文
	ctx := middleware.NewContext(w, r, g.logger)

	// 匹配路由
	route, params := g.router.Match(r)
	if route == nil {
		http.Error(w, "404 Not Found", http.StatusNotFound)
		return
	}

	// 设置路径参数
	ctx.Params = params

	// 获取上游服务
	upstream, ok := g.watcher.GetUpstream(route.UpstreamID)
	if !ok {
		http.Error(w, "503 Upstream Not Found", http.StatusServiceUnavailable)
		return
	}

	// 构建处理器链
	finalHandler := g.proxyHandler(upstream)
	handler := g.globalChain.Then(finalHandler)

	// 执行
	handler(ctx)
}

// proxyHandler 反向代理处理器
func (g *Gateway) proxyHandler(upstream *config.Upstream) middleware.HandlerFunc {
	return func(ctx *middleware.Context) {
		// 创建负载均衡器
		lb := balancer.NewLoadBalancer(upstream.Type, upstream)

		// 选择目标节点
		clientIP := ctx.Request.RemoteAddr
		target, err := lb.Select(clientIP)
		if err != nil {
			http.Error(ctx.Response, "503 No Healthy Target", http.StatusServiceUnavailable)
			return
		}

		// 增加活跃连接数
		upstream.IncrementActiveConns(target.Address)
		defer upstream.DecrementActiveConns(target.Address)

		// 构建目标 URL
		targetURL, _ := url.Parse("http://" + target.Address)

		// 创建反向代理
		proxy := httputil.NewSingleHostReverseProxy(targetURL)

		// 自定义错误处理
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			g.logger.Error("proxy error",
				zap.String("target", target.Address),
				zap.Error(err))
			http.Error(w, "502 Bad Gateway", http.StatusBadGateway)
		}

		// 修改请求
		proxy.Director = func(req *http.Request) {
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host
			req.Host = targetURL.Host

			// 添加 X-Forwarded 头
			if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
				req.Header.Set("X-Forwarded-For", clientIP)
			}
			req.Header.Set("X-Forwarded-Proto", "http")
		}

		// 执行代理
		proxy.ServeHTTP(ctx.Response, ctx.Request)
	}
}
