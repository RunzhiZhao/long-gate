package middleware

import (
	"time"
)

// RequestID 请求 ID 中间件
func RequestID() Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) {
			requestID := ctx.Request.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = generateRequestID()
			}
			ctx.Set("request_id", requestID)
			ctx.Response.Header().Set("X-Request-ID", requestID)
			next(ctx)
		}
	}
}

// generateRequestID 生成请求 ID（简化实现）
func generateRequestID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

func randomString(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = chars[time.Now().UnixNano()%int64(len(chars))]
	}
	return string(result)
}
