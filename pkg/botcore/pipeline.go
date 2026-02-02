package botcore

// StreamChunk 描述流式输出片段。
type StreamChunk struct {
	Content string
	Payload any // 扩展：支持携带复杂对象（如 TemplateCard），用于非流式回复
	IsFinal bool
}

// NoResponse 是一个哨兵值，用于标记不需要被动回复。
// 当 StreamChunk.Payload == NoResponse 时，Bot 层应直接返回 HTTP 200 OK 空包。
var NoResponse = struct{}{}

// PipelineInvoker 抽象命令/业务执行器。
type PipelineInvoker interface {
	Trigger(update RequestSnapshot) <-chan StreamChunk
}

// PipelineFunc 便于直接以函数充当 PipelineInvoker。
type PipelineFunc func(update RequestSnapshot) <-chan StreamChunk

// Trigger 实现 PipelineInvoker 接口。
func (f PipelineFunc) Trigger(update RequestSnapshot) <-chan StreamChunk {
	if f == nil {
		return nil
	}
	return f(update)
}
