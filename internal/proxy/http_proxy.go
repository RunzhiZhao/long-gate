package proxy

import (
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// HTTPProxy 是 HTTP 和 WebSocket 代理的实现
type HTTPProxy struct {
	proxy  *httputil.ReverseProxy
	target *url.URL
}

// NewHTTPProxy 创建一个新的 HTTP 代理实例
func NewHTTPProxy(targetURLStr string) (*HTTPProxy, error) {
	targetURL, err := url.Parse(targetURLStr)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// 自定义 Director 可以在转发前修改请求，如添加/删除 Header
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = targetURL.Host // 设置 Host 避免后端服务出错
		req.URL.Host = targetURL.Host
		req.URL.Scheme = targetURL.Scheme
		req.URL.Path = targetURL.Path + req.URL.Path // 路径拼接

		// 添加 X-Forwarded-For 头部
		if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
			if prior, ok := req.Header["X-Forwarded-For"]; ok {
				clientIP = strings.Join(prior, ", ") + ", " + clientIP
			}
			req.Header.Set("X-Forwarded-For", clientIP)
		}
	}

	// WebSocket 升级请求会被 ReverseProxy 自动处理

	return &HTTPProxy{
		proxy:  proxy,
		target: targetURL,
	}, nil
}

// ServeHTTP 实际执行代理转发
func (p *HTTPProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	targetHost := ""
	targetPath := r.URL.Path
	if p.target != nil {
		targetHost = p.target.Host
		targetPath = p.target.Path + r.URL.Path
	}
	log.Printf("Proxying HTTP request to: %s%s", targetHost, targetPath)
	p.proxy.ServeHTTP(w, r)
}
