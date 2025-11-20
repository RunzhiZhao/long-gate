package middleware

import "net/http"

// CORS 跨域中间件
func CORS() Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) {
			ctx.Response.Header().Set("Access-Control-Allow-Origin", "*")
			ctx.Response.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			ctx.Response.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			// 处理预检请求
			if ctx.Request.Method == "OPTIONS" {
				ctx.Response.WriteHeader(http.StatusOK)
				ctx.Abort()
				return
			}

			next(ctx)
		}
	}
}
