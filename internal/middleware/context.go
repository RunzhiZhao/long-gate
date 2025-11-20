package middleware

import (
	"net/http"

	"go.uber.org/zap"
)

// Context 请求上下文
type Context struct {
	Request  *http.Request
	Response http.ResponseWriter
	Params   map[string]string // 路径参数
	Data     map[string]any    // 中间件共享数据
	Logger   *zap.Logger
	aborted  bool
}

// NewContext 创建上下文
func NewContext(w http.ResponseWriter, r *http.Request, logger *zap.Logger) *Context {
	return &Context{
		Request:  r,
		Response: w,
		Params:   make(map[string]string),
		Data:     make(map[string]any),
		Logger:   logger,
		aborted:  false,
	}
}

// Abort 中止请求处理
func (c *Context) Abort() {
	c.aborted = true
}

// IsAborted 判断是否已中止
func (c *Context) IsAborted() bool {
	return c.aborted
}

// Set 设置共享数据
func (c *Context) Set(key string, value any) {
	c.Data[key] = value
}

// Get 获取共享数据
func (c *Context) Get(key string) (any, bool) {
	val, exists := c.Data[key]
	return val, exists
}
