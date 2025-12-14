package command

import "context"

// ChatOptions 定义调用 LLM 时的可选参数。
type ChatOptions struct {
	Model string // 指定使用的模型名称（在配置文件中定义的 name）
}

// ChatOption 是设置 ChatOptions 的函数类型。
type ChatOption func(*ChatOptions)

// WithModel 指定本次调用使用的模型。
func WithModel(name string) ChatOption {
	return func(o *ChatOptions) {
		o.Model = name
	}
}

// LLMProvider 定义 Command 层依赖的 AI 能力接口。
// 这使得 Command 可以调用 AI 服务，而无需直接依赖 pkg/ai 包。
type LLMProvider interface {
	// Chat 发起与 AI 的对话。
	// sessionID: 会话标识
	// prompt: 用户输入
	// opts: 可选配置（如指定模型）
	Chat(ctx context.Context, sessionID, prompt string, opts ...ChatOption) (<-chan string, error)
}
