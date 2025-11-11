package middleware

import (
	"net/http"

	"github.com/RunzhiZhao/long-gate/internal/config"
)

// HandlerFunc 定义中间件的核心处理函数签名
type HandlerFunc func(w http.ResponseWriter, r *http.Request) bool

// Middleware 接口定义所有中间件必须实现的方法
type Middleware interface {
	Name() string
	Process(w http.ResponseWriter, r *http.Request) bool // 返回 true 继续执行，返回 false 中断请求
}

// Chain 结构体持有所有需要执行的中间件
type Chain struct {
	middlewares []Middleware
}

// NewChain 创建一个新的中间件链
func NewChain(routeCfg config.RouteConfig) *Chain {
	c := &Chain{}

	// 根据配置加载和初始化中间件
	for _, mwCfg := range routeCfg.Middlewares {
		var mw Middleware

		switch mwCfg.Name {
		case "jwt":
			// ⚠️ 密钥应从配置中获取
			jwtSecret := config.GetGatewayConfig().JWTSecret
			mw = NewJWTMiddleware(jwtSecret)
		case "rate_limit":
			// ⚠️ 参数解析应更健壮，这里简化处理
			mw = NewRateLimitMiddleware(mwCfg.Param)
		// 可以添加更多的中间件...
		default:
			// 忽略未知的中间件
			continue
		}

		c.middlewares = append(c.middlewares, mw)
	}

	return c
}

// Execute 依次执行链中的所有中间件
func (c *Chain) Execute(w http.ResponseWriter, r *http.Request) bool {
	for _, mw := range c.middlewares {
		// 只要有一个中间件返回 false，就中断后续执行
		if !mw.Process(w, r) {
			return false
		}
	}
	return true
}
