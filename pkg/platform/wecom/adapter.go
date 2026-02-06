// Package wecom 提供企业微信平台的 botcore 适配层。
// 通过 wecomproto SDK 实现协议细节，本包负责 botcore 接口适配。
package wecom

import (
	"strings"

	"github.com/IMBotPlatform/IMBotCore/pkg/botcore"
	wecomproto "github.com/IMBotPlatform/bot-protocol-wecom/pkg/wecom"
)

// PipelineAdapter 将 botcore.PipelineInvoker 适配为 wecomproto.Handler。
type PipelineAdapter struct {
	pipeline botcore.PipelineInvoker
}

// NewPipelineAdapter 创建适配器。
func NewPipelineAdapter(pipeline botcore.PipelineInvoker) *PipelineAdapter {
	return &PipelineAdapter{pipeline: pipeline}
}

// Handle 实现 wecomproto.Handler 接口。
func (a *PipelineAdapter) Handle(ctx wecomproto.Context) <-chan wecomproto.Chunk {
	if a.pipeline == nil {
		return nil
	}

	// 构建 botcore 快照
	snapshot := buildSnapshot(ctx.Message, ctx.StreamID)

	// 创建 Responser 适配器
	responser := &BotResponser{bot: ctx.Bot}

	pipelineCtx := botcore.PipelineContext{
		Snapshot:  snapshot,
		Responser: responser,
	}

	// 触发 pipeline 并转换输出
	botcoreCh := a.pipeline.Trigger(pipelineCtx)
	if botcoreCh == nil {
		return nil
	}

	// 转换 botcore.StreamChunk 到 wecomproto.Chunk
	outCh := make(chan wecomproto.Chunk)
	go func() {
		defer close(outCh)
		for chunk := range botcoreCh {
			// 转换 NoResponse
			if chunk.Payload == botcore.NoResponse {
				outCh <- wecomproto.Chunk{Payload: wecomproto.NoResponse}
				continue
			}
			outCh <- wecomproto.Chunk{
				Content: chunk.Content,
				Payload: chunk.Payload,
				IsFinal: chunk.IsFinal,
			}
		}
	}()

	return outCh
}

// BotResponser 适配 wecomproto.Bot 为 botcore.Responser。
type BotResponser struct {
	bot *wecomproto.Bot
}

// Response 实现 botcore.Responser 接口。
func (r *BotResponser) Response(responseURL string, msg any) error {
	if r.bot == nil {
		return nil
	}
	return r.bot.Response(responseURL, msg)
}

// ResponseMarkdown 实现 botcore.Responser 接口。
func (r *BotResponser) ResponseMarkdown(responseURL, content string) error {
	if r.bot == nil {
		return nil
	}
	return r.bot.ResponseMarkdown(responseURL, content)
}

// ResponseTemplateCard 实现 botcore.Responser 接口。
func (r *BotResponser) ResponseTemplateCard(responseURL string, card any) error {
	if r.bot == nil {
		return nil
	}
	typedCard, ok := card.(*wecomproto.TemplateCard)
	if !ok {
		return nil
	}
	return r.bot.ResponseTemplateCard(responseURL, typedCard)
}

// buildSnapshot 将 wecomproto.Message 转换为 botcore.RequestSnapshot。
func buildSnapshot(msg *wecomproto.Message, streamID string) botcore.RequestSnapshot {
	if msg == nil {
		return botcore.RequestSnapshot{ID: streamID}
	}

	text := ""
	if msg.Text != nil {
		text = msg.Text.Content
	}

	meta := map[string]string{
		"platform":     "wecom",
		"msgtype":      msg.MsgType,
		"response_url": msg.ResponseURL,
	}
	if msg.Stream != nil {
		meta["stream_id"] = msg.Stream.ID
	}

	attachments := make([]botcore.Attachment, 0)
	addAttachment := func(attType botcore.AttachmentType, url string, data []byte) {
		if url == "" && len(data) == 0 {
			return
		}
		attachments = append(attachments, botcore.Attachment{Type: attType, URL: url, Data: data})
	}

	switch msg.MsgType {
	case "image":
		if msg.Image != nil {
			addAttachment(botcore.AttachmentTypeImage, msg.Image.URL, msg.Image.Data)
		}
	case "file":
		if msg.File != nil {
			addAttachment(botcore.AttachmentTypeFile, msg.File.URL, nil)
		}
	case "mixed":
		if msg.Mixed != nil {
			var textParts []string
			for _, item := range msg.Mixed.Items {
				switch item.MsgType {
				case "image":
					if item.Image != nil {
						addAttachment(botcore.AttachmentTypeImage, item.Image.URL, item.Image.Data)
					}
				case "text":
					if item.Text != nil && item.Text.Content != "" {
						textParts = append(textParts, item.Text.Content)
					}
				}
			}
			// 如果 mixed 消息中有 text，合并到 text 字段
			if len(textParts) > 0 {
				text = strings.Join(textParts, "\n")
			}
		}
	}

	return botcore.RequestSnapshot{
		ID:          streamID,
		SenderID:    msg.From.UserID,
		ChatID:      msg.ChatID,
		ChatType:    mapWecomChatType(msg.ChatType),
		Text:        text,
		Attachments: attachments,
		Raw:         msg,
		ResponseURL: msg.ResponseURL,
		Metadata:    meta,
	}
}

// mapWecomChatType 将企业微信 chattype 规范化为内部标准类型。
func mapWecomChatType(raw string) botcore.ChatType {
	switch raw {
	case "single":
		return botcore.ChatTypeSingle
	case "group", "chatroom":
		return botcore.ChatTypeChatroom
	default:
		return botcore.ChatType(raw)
	}
}
