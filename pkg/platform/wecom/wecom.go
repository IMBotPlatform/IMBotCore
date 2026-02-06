// Package wecom 提供企业微信平台的 botcore 适配层。
// 通过 wecomproto SDK 实现协议细节，本包负责 botcore 接口适配。
package wecom

import (
	"time"

	"github.com/IMBotPlatform/IMBotCore/pkg/botcore"
	wecomproto "github.com/IMBotPlatform/bot-protocol-wecom/pkg/wecom"
)

// Bot 是对 wecomproto.Bot 的包装，支持 botcore.PipelineInvoker。
type Bot struct {
	*wecomproto.Bot
}

// StartOptions 直接使用 wecomproto 的启动选项。
type StartOptions = wecomproto.StartOptions

// NewBot 创建集成 botcore.PipelineInvoker 的企业微信 Bot。
// Parameters:
//   - token: 企业微信配置的消息校验 Token
//   - encodingAESKey: 企业微信后台生成的 43 字节 Base64 编码字符串
//   - corpID: 企业 ID，用于校验消息归属
//   - streamMsgTTL: 流式会话最大存活时间（<=0 时使用默认值）
//   - streamWaitTimeout: 刷新请求等待流水线片段的最大时长（<=0 时使用默认值）
//   - pipeline: 首包触发的业务流水线实现，可为 nil
//
// Returns:
//   - *Bot: 成功初始化的 Bot 实例
//   - error: 当加解密上下文初始化失败时返回错误
func NewBot(token, encodingAESKey, corpID string, streamMsgTTL, streamWaitTimeout time.Duration, pipeline botcore.PipelineInvoker) (*Bot, error) {
	// 将 pipeline 适配为 wecomproto.Handler
	adapter := NewPipelineAdapter(pipeline)

	// 使用 wecomproto SDK 创建底层 Bot
	bot, err := wecomproto.NewBotWithOptions(token, encodingAESKey, corpID, streamMsgTTL, streamWaitTimeout, adapter)
	if err != nil {
		return nil, err
	}

	return &Bot{Bot: bot}, nil
}

// 以下类型别名方便外部使用，避免直接导入 wecomproto
type (
	Message             = wecomproto.Message
	MessageSender       = wecomproto.MessageSender
	TemplateCard        = wecomproto.TemplateCard
	EncryptedRequest    = wecomproto.EncryptedRequest
	EncryptedResponse   = wecomproto.EncryptedResponse
	MarkdownMessage     = wecomproto.MarkdownMessage
	MarkdownPayload     = wecomproto.MarkdownPayload
	TemplateCardMessage = wecomproto.TemplateCardMessage
	TextPayload         = wecomproto.TextPayload
	StreamPayload       = wecomproto.StreamPayload
)

// NewCrypt 创建加解密器（委托给 wecomproto）。
func NewCrypt(token, encodingAESKey, corpID string) (*wecomproto.Crypt, error) {
	return wecomproto.NewCrypt(token, encodingAESKey, corpID)
}

// CalcSignature 计算签名（委托给 wecomproto）。
func CalcSignature(token, timestamp, nonce, data string) string {
	return wecomproto.CalcSignature(token, timestamp, nonce, data)
}

// BuildStreamReply 构建流式回复（委托给 wecomproto）。
func BuildStreamReply(streamID, content string, finish bool) wecomproto.StreamReply {
	return wecomproto.BuildStreamReply(streamID, content, finish)
}

// Response 实现 botcore.Responser 接口。
func (b *Bot) Response(responseURL string, msg any) error {
	return b.Bot.Response(responseURL, msg)
}

// ResponseMarkdown 实现 botcore.Responser 接口。
func (b *Bot) ResponseMarkdown(responseURL, content string) error {
	return b.Bot.ResponseMarkdown(responseURL, content)
}

// ResponseTemplateCard 实现 botcore.Responser 接口。
func (b *Bot) ResponseTemplateCard(responseURL string, card any) error {
	typedCard, ok := card.(*wecomproto.TemplateCard)
	if !ok {
		return nil
	}
	return b.Bot.ResponseTemplateCard(responseURL, typedCard)
}
