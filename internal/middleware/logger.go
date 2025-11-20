package middleware

import (
	"time"

	"go.uber.org/zap"
)

// Logger 日志中间件
func Logger(logger *zap.Logger) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) {
			start := time.Now()
			path := ctx.Request.URL.Path
			method := ctx.Request.Method

			next(ctx)

			latency := time.Since(start)

			logger.Info("request handled",
				zap.String("method", method),
				zap.String("path", path),
				zap.Duration("latency", latency),
				zap.String("client_ip", ctx.Request.RemoteAddr),
			)
		}
	}
}
