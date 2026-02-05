package botcore

// Matcher 定义路由匹配逻辑。
// 返回 true 表示该路由应该处理此首包快照。
type Matcher func(update RequestSnapshot) bool

// Route 定义单条路由规则。
type Route struct {
	Name    string
	Matcher Matcher
	Handler PipelineInvoker
}

// Chain 实现了一个基于责任链/路由表的 PipelineInvoker。
// 它按顺序检查路由，一旦匹配成功，就移交给对应的 PipelineInvoker，并停止后续匹配。
// 如果所有路由都不匹配，且设置了 defaultHandler，则调用 defaultHandler。
type Chain struct {
	routes         []Route
	defaultHandler PipelineInvoker
}

// NewChain 创建一个新的责任链路由器。
// Parameters:
//   - defaultHandler: 默认处理器；为 nil 表示无默认处理
//
// Returns:
//   - *Chain: 初始化后的责任链路由器
func NewChain(defaultHandler PipelineInvoker) *Chain {
	return &Chain{
		routes:         make([]Route, 0),
		defaultHandler: defaultHandler,
	}
}

// AddRoute 添加一条路由规则。
// Parameters:
//   - name: 路由名称（便于调试与日志）
//   - matcher: 匹配规则
//   - handler: 命中后执行的 PipelineInvoker
func (c *Chain) AddRoute(name string, matcher Matcher, handler PipelineInvoker) {
	c.routes = append(c.routes, Route{
		Name:    name,
		Matcher: matcher,
		Handler: handler,
	})
}

// Trigger 实现 PipelineInvoker 接口。
// Parameters:
//   - ctx: Pipeline 执行上下文（包含 Snapshot 与 Responser）
//
// Returns:
//   - <-chan StreamChunk: 流式输出片段通道（无匹配时可能返回 nil）
func (c *Chain) Trigger(ctx PipelineContext) <-chan StreamChunk {
	update := ctx.Snapshot
	// 1. 遍历路由表
	for _, route := range c.routes {
		if route.Matcher(update) {
			// 匹配成功，移交控制权
			return route.Handler.Trigger(ctx)
		}
	}

	// 2. 没有任何匹配，使用默认处理器
	if c.defaultHandler != nil {
		return c.defaultHandler.Trigger(ctx)
	}

	// 3. 既无匹配也无默认处理器，返回空流 (静默)
	return nil
}

// ContextMatcher 辅助函数：创建一个基于上下文的 Matcher (预留接口，目前快照中主要是 Text)
// 这里提供一些常用的 Matcher 构造器

// MatchPrefix 返回一个匹配文本前缀的 Matcher。
// Parameters:
//   - prefix: 需要匹配的文本前缀
//
// Returns:
//   - Matcher: 当前前缀匹配器
func MatchPrefix(prefix string) Matcher {
	return func(u RequestSnapshot) bool {
		return len(u.Text) >= len(prefix) && u.Text[0:len(prefix)] == prefix
	}
}

// MatchAny 返回一个总是匹配的 Matcher。
// Returns:
//   - Matcher: 永远返回 true 的匹配器
func MatchAny() Matcher {
	return func(u RequestSnapshot) bool {
		return true
	}
}
