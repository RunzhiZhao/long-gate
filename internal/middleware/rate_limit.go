package middleware

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// IP 限流器存储 (生产环境应使用分布式缓存如 Redis)
var ipLimiters = make(map[string]*rate.Limiter)
var limiterMutex sync.Mutex

type RateLimitMiddleware struct {
	rate  rate.Limit // 令牌填充速率
	burst int        // 令牌桶容量
}

func NewRateLimitMiddleware(param string) *RateLimitMiddleware {
	// 简化： param "10/s" -> rate=10, burst=10
	var r rate.Limit = 10
	var b = 10
	// ⚠️ 生产代码应解析 param 字符串，这里使用硬编码简化 MVP

	return &RateLimitMiddleware{
		rate:  r,
		burst: b,
	}
}

func (r *RateLimitMiddleware) Name() string {
	return "rate_limit"
}

// Process 执行限流检查
func (r *RateLimitMiddleware) Process(w http.ResponseWriter, req *http.Request) bool {
	// 使用客户端 IP 地址作为限流 Key
	ip := req.RemoteAddr

	limiter := getLimiter(ip, r.rate, r.burst)

	if !limiter.Allow() {
		w.Header().Set("X-Rate-Limit-Retry-After", "1")
		http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
		return false
	}

	return true
}

// getLimiter 获取或创建 IP 对应的限流器
func getLimiter(ip string, r rate.Limit, b int) *rate.Limiter {
	limiterMutex.Lock()
	defer limiterMutex.Unlock()

	limiter, exists := ipLimiters[ip]
	if !exists {
		// 创建一个新的限流器
		limiter = rate.NewLimiter(r, b)
		ipLimiters[ip] = limiter

		// ⚠️ 简单清理机制：10分钟后删除不活跃的 IP
		go func() {
			time.Sleep(10 * time.Minute)
			limiterMutex.Lock()
			delete(ipLimiters, ip)
			limiterMutex.Unlock()
		}()
	}
	return limiter
}
