package command

import (
	"context"
	"fmt"

	"github.com/IMBotPlatform/IMBotCore/botcore"
)

// keyExecutionContext 是 context.Context 中存储 ExecutionContext 的键。
type keyExecutionContext struct{}

// ContextValues 存储命令执行过程中的上下文扩展字段。
type ContextValues map[string]string

// ConversationStore 定义上下文存取接口，便于替换实现。
type ConversationStore interface {
	Load(key string) (ContextValues, error)
	Save(key string, values ContextValues) error
}

// ExecutionContext 为命令 handler 提供必要的环境信息。
type ExecutionContext struct {
	Update          botcore.Update
	StreamID        string
	Values          ContextValues
	Store           ConversationStore
	llm             LLMProvider
	responder       botcore.ActiveResponder
	
	// sendSignal 是一个回调函数，允许 Command 立即向 Pipeline 发送信号
	sendSignal func(chunk botcore.StreamChunk)
}

// SetResponsePayload 立即发送非流式响应对象。
func (ctx *ExecutionContext) SetResponsePayload(payload interface{}) {
	if ctx.sendSignal != nil {
		ctx.sendSignal(botcore.StreamChunk{
			Payload: payload,
			IsFinal: true,
		})
	}
}

// SetNoResponse 立即发送静默信号。
// Bot 层收到此信号后将直接返回 HTTP 200 OK 空包。
func (ctx *ExecutionContext) SetNoResponse() {
	if ctx.sendSignal != nil {
		ctx.sendSignal(botcore.StreamChunk{
			Payload: botcore.NoResponse,
			IsFinal: true,
		})
	}
}

// LLM 返回 AI 服务提供者。
func (ctx *ExecutionContext) LLM() LLMProvider {
	return ctx.llm
}

// Responder 返回主动消息发送器。
func (ctx *ExecutionContext) Responder() botcore.ActiveResponder {
	return ctx.responder
}

// ConversationKey 返回当前上下文在存储中的唯一 key。
func (ctx *ExecutionContext) ConversationKey() string {
	if ctx == nil {
		return ""
	}
	return fmt.Sprintf("%s:%s", ctx.Update.ChatID, ctx.Update.SenderID)
}

// WithExecutionContext 将 ExecutionContext 注入到标准 context.Context 中。
func WithExecutionContext(ctx context.Context, execCtx *ExecutionContext) context.Context {
	return context.WithValue(ctx, keyExecutionContext{}, execCtx)
}

// FromContext 从标准 context.Context 中提取 ExecutionContext。
func FromContext(ctx context.Context) *ExecutionContext {
	val := ctx.Value(keyExecutionContext{})
	if val == nil {
		return nil
	}
	return val.(*ExecutionContext)
}
