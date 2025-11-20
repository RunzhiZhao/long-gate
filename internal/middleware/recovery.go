package middleware

import (
	"net/http"

	"go.uber.org/zap"
)

// Recovery 恢复中间件（捕获 panic）
func Recovery(logger *zap.Logger) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("panic recovered",
						zap.Any("error", err),
						zap.String("path", ctx.Request.URL.Path),
					)
					ctx.Response.WriteHeader(http.StatusInternalServerError)
					ctx.Response.Write([]byte("Internal Server Error"))
					ctx.Abort()
				}
			}()
			next(ctx)
		}
	}
}
