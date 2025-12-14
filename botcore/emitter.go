package botcore

// Emitter 将流水线产生的流式片段转换为平台可用的响应。
type Emitter interface {
	Encode(update Update, streamID string, chunk StreamChunk) (interface{}, error)
}

// EmitterFunc 允许直接用函数实现。
type EmitterFunc func(update Update, streamID string, chunk StreamChunk) (interface{}, error)

// Encode 实现 Emitter 接口。
func (f EmitterFunc) Encode(update Update, streamID string, chunk StreamChunk) (interface{}, error) {
	if f == nil {
		return nil, nil
	}
	return f(update, streamID, chunk)
}
