package router

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// RouteStatus 路由状态
type RouteStatus int

const (
	RouteStatusDisabled RouteStatus = 0
	RouteStatusEnabled  RouteStatus = 1
)

// Route 路由规则定义
type Route struct {
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	Priority   int              `json:"priority"` // 优先级，数字越大越优先
	Status     RouteStatus      `json:"status"`
	Predicates *RoutePredicates `json:"predicates"`
	UpstreamID string           `json:"upstream_id"`
	Plugins    map[string]any   `json:"plugins,omitempty"`
	Version    int64            `json:"version"` // 配置版本号
	CreateTime int64            `json:"create_time"`
	UpdateTime int64            `json:"update_time"`
}

// RoutePredicates 路由匹配谓词
type RoutePredicates struct {
	// 路径匹配
	Path      string         `json:"path"`      // 如: /api/users/:id
	PathType  PathType       `json:"path_type"` // prefix/exact/regex
	PathRegex *regexp.Regexp `json:"-"`         // 编译后的正则

	// HTTP 方法
	Methods []string `json:"methods,omitempty"` // ["GET", "POST"]

	// 请求头匹配
	Headers map[string]string `json:"headers,omitempty"` // {"X-API-Key": "xxx"}

	// Host 匹配
	Hosts []string `json:"hosts,omitempty"` // ["api.example.com"]

	// 查询参数匹配
	QueryParams map[string]string `json:"query_params,omitempty"`
}

// PathType 路径匹配类型
type PathType string

const (
	PathTypePrefix PathType = "prefix" // 前缀匹配 (默认)
	PathTypeExact  PathType = "exact"  // 精确匹配
	PathTypeRegex  PathType = "regex"  // 正则匹配
)

// Validate 验证路由配置
func (r *Route) Validate() error {
	if r.ID == "" {
		return fmt.Errorf("route id cannot be empty")
	}
	if r.Predicates == nil {
		return fmt.Errorf("route predicates cannot be nil")
	}
	if r.Predicates.Path == "" {
		return fmt.Errorf("route path cannot be empty")
	}
	if r.UpstreamID == "" {
		return fmt.Errorf("upstream_id cannot be empty")
	}

	// 验证并编译正则表达式
	if r.Predicates.PathType == PathTypeRegex {
		regex, err := regexp.Compile(r.Predicates.Path)
		if err != nil {
			return fmt.Errorf("invalid path regex: %w", err)
		}
		r.Predicates.PathRegex = regex
	}

	// 验证 HTTP 方法
	for _, method := range r.Predicates.Methods {
		method = strings.ToUpper(method)
		if method != "GET" && method != "POST" && method != "PUT" &&
			method != "DELETE" && method != "PATCH" && method != "HEAD" &&
			method != "OPTIONS" {
			return fmt.Errorf("invalid http method: %s", method)
		}
	}

	return nil
}

// Match 判断请求是否匹配该路由
func (r *Route) Match(path, method, host string, headers map[string]string) bool {
	if r.Status != RouteStatusEnabled {
		return false
	}

	// 1. 路径匹配
	if !r.matchPath(path) {
		return false
	}

	// 2. 方法匹配
	if !r.matchMethod(method) {
		return false
	}

	// 3. Host 匹配
	if !r.matchHost(host) {
		return false
	}

	// 4. Header 匹配
	if !r.matchHeaders(headers) {
		return false
	}

	return true
}

func (r *Route) matchPath(path string) bool {
	switch r.Predicates.PathType {
	case PathTypeExact:
		return path == r.Predicates.Path
	case PathTypeRegex:
		if r.Predicates.PathRegex == nil {
			return false
		}
		return r.Predicates.PathRegex.MatchString(path)
	case PathTypePrefix:
		fallthrough
	default:
		return strings.HasPrefix(path, r.Predicates.Path)
	}
}

func (r *Route) matchMethod(method string) bool {
	if len(r.Predicates.Methods) == 0 {
		return true // 未指定方法，匹配所有
	}
	method = strings.ToUpper(method)
	for _, m := range r.Predicates.Methods {
		if strings.ToUpper(m) == method {
			return true
		}
	}
	return false
}

func (r *Route) matchHost(host string) bool {
	if len(r.Predicates.Hosts) == 0 {
		return true
	}
	for _, h := range r.Predicates.Hosts {
		if h == host || h == "*" {
			return true
		}
	}
	return false
}

func (r *Route) matchHeaders(headers map[string]string) bool {
	if len(r.Predicates.Headers) == 0 {
		return true
	}
	for key, expectedValue := range r.Predicates.Headers {
		actualValue, exists := headers[key]
		if !exists || actualValue != expectedValue {
			return false
		}
	}
	return true
}

// ToJSON 序列化为 JSON
func (r *Route) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// FromJSON 从 JSON 反序列化
func (r *Route) FromJSON(data []byte) error {
	if err := json.Unmarshal(data, r); err != nil {
		return err
	}
	return r.Validate()
}
