package middleware

// HandlerFunc 处理函数
type HandlerFunc func(*Context)

// Middleware 中间件函数
type Middleware func(HandlerFunc) HandlerFunc

// Chain 中间件链
type Chain struct {
	middlewares []Middleware
}

// NewChain 创建中间件链
func NewChain(middlewares ...Middleware) *Chain {
	return &Chain{
		middlewares: middlewares,
	}
}

// Then 应用中间件链到最终处理器
func (c *Chain) Then(final HandlerFunc) HandlerFunc {
	// 从后往前包装
	handler := final
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		handler = c.middlewares[i](handler)
	}
	return handler
}

// Append 追加中间件
func (c *Chain) Append(m ...Middleware) *Chain {
	newChain := &Chain{
		middlewares: make([]Middleware, len(c.middlewares)+len(m)),
	}
	copy(newChain.middlewares, c.middlewares)
	copy(newChain.middlewares[len(c.middlewares):], m)
	return newChain
}
