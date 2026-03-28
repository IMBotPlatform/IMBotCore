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
	snapshot := buildSnapshot(ctx)

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

// buildSnapshot 将 wecomproto.Context 转换为 botcore.RequestSnapshot。
func buildSnapshot(ctx wecomproto.Context) botcore.RequestSnapshot {
	msg := ctx.Message
	streamID := ctx.StreamID
	if msg == nil {
		return botcore.RequestSnapshot{ID: streamID}
	}

	meta := map[string]string{
		"platform":     "wecom",
		"msgtype":      msg.MsgType,
		"response_url": msg.ResponseURL,
	}
	if msg.Stream != nil {
		meta["stream_id"] = msg.Stream.ID
	}

	return botcore.RequestSnapshot{
		ID:          streamID,
		SenderID:    msg.From.UserID,
		ChatID:      msg.ChatID,
		ChatType:    mapWecomChatType(msg.ChatType),
		Text:        extractMessageText(msg),
		Attachments: collectMessageAttachments(msg, ctx),
		Reference:   buildReference(msg.Quote, ctx),
		Raw:         msg,
		ResponseURL: msg.ResponseURL,
		Metadata:    meta,
	}
}

// buildReference 将企业微信 quote 转换为 botcore.Reference。
func buildReference(quote *wecomproto.QuotePayload, ctx wecomproto.Context) *botcore.Reference {
	if quote == nil {
		return nil
	}

	return &botcore.Reference{
		Type:        quote.MsgType,
		Text:        extractQuoteText(quote),
		Attachments: collectQuoteAttachments(quote, ctx),
		Raw:         quote,
		Metadata: map[string]string{
			"platform": "wecom",
			"msgtype":  quote.MsgType,
		},
	}
}

// extractMessageText 提取消息中的主要文本。
func extractMessageText(msg *wecomproto.Message) string {
	if msg == nil {
		return ""
	}

	switch msg.MsgType {
	case "text":
		if msg.Text != nil {
			return msg.Text.Content
		}
	case "voice":
		if msg.Voice != nil {
			return msg.Voice.Content
		}
	case "mixed":
		return extractMixedText(msg.Mixed)
	}

	return ""
}

// extractQuoteText 提取引用消息中的主要文本。
func extractQuoteText(quote *wecomproto.QuotePayload) string {
	if quote == nil {
		return ""
	}

	switch quote.MsgType {
	case "text":
		if quote.Text != nil {
			return quote.Text.Content
		}
	case "voice":
		if quote.Voice != nil {
			return quote.Voice.Content
		}
	case "mixed":
		return extractMixedText(quote.Mixed)
	}

	return ""
}

// extractMixedText 提取 mixed 中的所有文本子项。
func extractMixedText(mixed *wecomproto.MixedPayload) string {
	if mixed == nil {
		return ""
	}

	textParts := make([]string, 0)
	for _, item := range mixed.Items {
		if item.MsgType == "text" && item.Text != nil && item.Text.Content != "" {
			textParts = append(textParts, item.Text.Content)
		}
	}
	return strings.Join(textParts, "\n")
}

// collectMessageAttachments 提取主消息中的附件。
func collectMessageAttachments(msg *wecomproto.Message, ctx wecomproto.Context) []botcore.Attachment {
	if msg == nil {
		return nil
	}

	attachments := make([]botcore.Attachment, 0)
	appendAttachment := func(att botcore.Attachment, ok bool) {
		if ok {
			attachments = append(attachments, att)
		}
	}

	switch msg.MsgType {
	case "image":
		appendAttachment(buildImageAttachment(msg.Image, ctx))
	case "file":
		appendAttachment(buildFileAttachment(msg.File, ctx))
	case "video":
		appendAttachment(buildVideoAttachment(msg.Video, ctx))
	case "mixed":
		if msg.Mixed != nil {
			for _, item := range msg.Mixed.Items {
				if item.MsgType == "image" {
					appendAttachment(buildImageAttachment(item.Image, ctx))
				}
			}
		}
	}

	return attachments
}

// collectQuoteAttachments 提取引用消息中的附件。
func collectQuoteAttachments(quote *wecomproto.QuotePayload, ctx wecomproto.Context) []botcore.Attachment {
	if quote == nil {
		return nil
	}

	attachments := make([]botcore.Attachment, 0)
	appendAttachment := func(att botcore.Attachment, ok bool) {
		if ok {
			attachments = append(attachments, att)
		}
	}

	switch quote.MsgType {
	case "image":
		appendAttachment(buildImageAttachment(quote.Image, ctx))
	case "file":
		appendAttachment(buildFileAttachment(quote.File, ctx))
	case "video":
		appendAttachment(buildVideoAttachment(quote.Video, ctx))
	case "mixed":
		if quote.Mixed != nil {
			for _, item := range quote.Mixed.Items {
				if item.MsgType == "image" {
					appendAttachment(buildImageAttachment(item.Image, ctx))
				}
			}
		}
	}

	return attachments
}

// buildImageAttachment 构造标准化图片附件。
func buildImageAttachment(img *wecomproto.ImagePayload, ctx wecomproto.Context) (botcore.Attachment, bool) {
	if img == nil || (img.URL == "" && len(img.Data) == 0) {
		return botcore.Attachment{}, false
	}
	return botcore.Attachment{
		Type:              botcore.AttachmentTypeImage,
		URL:               img.URL,
		Data:              img.Data,
		DownloadTransform: buildAttachmentDownloadTransform(img.AESKey, ctx),
	}, true
}

// buildFileAttachment 构造标准化文件附件。
func buildFileAttachment(file *wecomproto.FilePayload, ctx wecomproto.Context) (botcore.Attachment, bool) {
	if file == nil || file.URL == "" {
		return botcore.Attachment{}, false
	}
	return botcore.Attachment{
		Type:              botcore.AttachmentTypeFile,
		URL:               file.URL,
		DownloadTransform: buildAttachmentDownloadTransform(file.AESKey, ctx),
	}, true
}

// buildVideoAttachment 构造标准化视频附件。
func buildVideoAttachment(video *wecomproto.VideoPayload, ctx wecomproto.Context) (botcore.Attachment, bool) {
	if video == nil || video.URL == "" {
		return botcore.Attachment{}, false
	}
	return botcore.Attachment{
		Type:              botcore.AttachmentTypeVideo,
		URL:               video.URL,
		DownloadTransform: buildAttachmentDownloadTransform(video.AESKey, ctx),
	}, true
}

// buildAttachmentDownloadTransform 为附件生成延迟解密逻辑。
func buildAttachmentDownloadTransform(resourceAESKey string, ctx wecomproto.Context) botcore.AttachmentDownloadTransform {
	if strings.TrimSpace(resourceAESKey) != "" {
		key := resourceAESKey
		return func(downloaded []byte) ([]byte, error) {
			return wecomproto.DecryptDownloadedFileWithAESKey(key, downloaded)
		}
	}

	if ctx.Bot != nil {
		return func(downloaded []byte) ([]byte, error) {
			return ctx.Bot.DecryptDownloadedFile(downloaded)
		}
	}

	return nil
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
