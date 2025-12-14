package botcore

// Matcher 定义路由匹配逻辑。
// 返回 true 表示该路由应该处理此 Update。
type Matcher func(update Update) bool

// Handler 定义路由处理逻辑。
// 实际上就是 PipelineInvoker，为了语义清晰起见定义别名。
type Handler PipelineInvoker

// Route 定义单条路由规则。
type Route struct {
	Name    string
	Matcher Matcher
	Handler Handler
}

// Chain 实现了一个基于责任链/路由表的 PipelineInvoker。
// 它按顺序检查路由，一旦匹配成功，就移交给对应的 Handler，并停止后续匹配。
// 如果所有路由都不匹配，且设置了 DefaultHandler，则调用 DefaultHandler。
type Chain struct {
	routes         []Route
	defaultHandler Handler
}

// NewChain 创建一个新的责任链路由器。
func NewChain(defaultHandler Handler) *Chain {
	return &Chain{
		routes:         make([]Route, 0),
		defaultHandler: defaultHandler,
	}
}

// AddRoute 添加一条路由规则。
func (c *Chain) AddRoute(name string, matcher Matcher, handler Handler) {
	c.routes = append(c.routes, Route{
		Name:    name,
		Matcher: matcher,
		Handler: handler,
	})
}

// Trigger 实现 PipelineInvoker 接口。
func (c *Chain) Trigger(update Update, streamID string) <-chan StreamChunk {
	// 1. 遍历路由表
	for _, route := range c.routes {
		if route.Matcher(update) {
			// 匹配成功，移交控制权
			return route.Handler.Trigger(update, streamID)
		}
	}

	// 2. 没有任何匹配，使用默认处理器
	if c.defaultHandler != nil {
		return c.defaultHandler.Trigger(update, streamID)
	}

	// 3. 既无匹配也无默认处理器，返回空流 (静默)
	return nil
}

// ContextMatcher 辅助函数：创建一个基于上下文的 Matcher (预留接口，目前 Update 中主要是 Text)
// 这里提供一些常用的 Matcher 构造器

// MatchPrefix 返回一个匹配文本前缀的 Matcher。
func MatchPrefix(prefix string) Matcher {
	return func(u Update) bool {
		return len(u.Text) >= len(prefix) && u.Text[0:len(prefix)] == prefix
	}
}

// MatchAny 返回一个总是匹配的 Matcher。
func MatchAny() Matcher {
	return func(u Update) bool {
		return true
	}
}
