package middleware

import (
	"net/http"
	"time"
)

// Timeout 超时中间件
func Timeout(timeout time.Duration) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) {
			done := make(chan struct{})

			go func() {
				next(ctx)
				close(done)
			}()

			select {
			case <-done:
				return
			case <-time.After(timeout):
				ctx.Response.WriteHeader(http.StatusGatewayTimeout)
				ctx.Response.Write([]byte("Gateway Timeout"))
				ctx.Abort()
			}
		}
	}
}
