package command

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/IMBotPlatform/IMBotCore/pkg/botcore"
)

// keyExecutionContext 是 context.Context 中存储 ExecutionContext 的键。
type keyExecutionContext struct{}

var (
	errExecutionContextNil     = errors.New("execution context is nil")
	errResponseURLEmpty        = errors.New("response_url is empty")
	errSendFuncMissing         = errors.New("send function is nil")
	errSendMarkdownMissing     = errors.New("send markdown function is nil")
	errSendTemplateCardMissing = errors.New("send template card function is nil")
)

// ExecutionContext 为命令 handler 提供必要的环境信息。
type ExecutionContext struct {
	RequestSnapshot botcore.RequestSnapshot

	// ch 是当前命令输出的流式通道。
	ch        chan<- botcore.StreamChunk
	finalOnce sync.Once // 保证 相同信息只发送一次。

	// responsers 由 Manager 注入，负责主动推送。
	responser botcore.Responser
}

// Response 发送主动回复消息。
// Parameters:
//   - msg: 平台消息负载
//
// Returns:
//   - error: 发送失败时返回
func (ctx *ExecutionContext) Response(msg any) error {
	responseURL, err := ctx.responseURL()
	if err != nil {
		return err
	}
	if ctx.responser == nil {
		return errSendFuncMissing
	}
	return ctx.responser.Response(responseURL, msg)
}

// ResponseMarkdown 发送 Markdown 主动回复。
// Parameters:
//   - content: Markdown 文本内容
//
// Returns:
//   - error: 发送失败时返回
func (ctx *ExecutionContext) ResponseMarkdown(content string) error {
	responseURL, err := ctx.responseURL()
	if err != nil {
		return err
	}
	if ctx.responser == nil {
		return errSendMarkdownMissing
	}
	return ctx.responser.ResponseMarkdown(responseURL, content)
}

// ResponseTemplateCard 发送模板卡片主动回复。
// Parameters:
//   - card: 模板卡片负载
//
// Returns:
//   - error: 发送失败时返回
func (ctx *ExecutionContext) ResponseTemplateCard(card any) error {
	responseURL, err := ctx.responseURL()
	if err != nil {
		return err
	}
	if ctx.responser == nil {
		return errSendTemplateCardMissing
	}
	return ctx.responser.ResponseTemplateCard(responseURL, card)
}

// SendPayload 立即发送非流式响应对象。
func (ctx *ExecutionContext) SendPayload(payload any) {
	ctx.sendFinal(botcore.StreamChunk{
		Payload: payload,
	})
}

// SendNoResponse 立即发送静默信号。
// Bot 层收到此信号后将直接返回 HTTP 200 OK 空包。
func (ctx *ExecutionContext) SendNoResponse() {
	ctx.sendFinal(botcore.StreamChunk{
		Payload: botcore.NoResponse,
	})
}

func (ctx *ExecutionContext) sendFinal(chunk botcore.StreamChunk) {
	if ctx.ch == nil {
		return
	}

	chunk.IsFinal = true
	ctx.finalOnce.Do(func() {
		ctx.ch <- chunk
	})
}

func (ctx *ExecutionContext) responseURL() (string, error) {
	if ctx == nil {
		return "", errExecutionContextNil
	}
	responseURL := strings.TrimSpace(ctx.RequestSnapshot.ResponseURL)
	if responseURL == "" {
		return "", errResponseURLEmpty
	}
	return responseURL, nil
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
