package wecom

import (
	"errors"
	"strings"

	"github.com/IMBotPlatform/IMBotCore/pkg/botcore"
)

// MessageAdapter 将企业微信 Message 映射为通用 Update。
type MessageAdapter struct{}

// Normalize 实现 botcore.Adapter。
func (MessageAdapter) Normalize(raw interface{}) (botcore.Update, error) {
	msg, ok := raw.(*Message)
	if !ok || msg == nil {
		return botcore.Update{}, errors.New("invalid wecom message")
	}

	text := ""
	if msg.Text != nil {
		text = msg.Text.Content
	}
	// 关键步骤：若文本以 @ 开头，剥离前置的 @ 提及，避免命令路由失败。
	text = stripLeadingMentions(text)

	meta := map[string]string{
		"platform":     "wecom",
		"msgtype":      msg.MsgType,
		"response_url": msg.ResponseURL,
	}
	if msg.Stream != nil {
		meta["stream_id"] = msg.Stream.ID
	}

	// 处理事件类型
	if msg.MsgType == "event" && msg.Event != nil {
		meta["event_type"] = msg.Event.EventType

		if msg.Event.EnterChat != nil {
			// 进入会话事件
			text = "/welcome" // 作为一个隐式命令，或者留空由 Pipeline 显式处理 event_type
		} else if msg.Event.TemplateCardEvent != nil {
			// 模板卡片事件
			cardEvent := msg.Event.TemplateCardEvent
			meta["card_type"] = cardEvent.CardType
			meta["event_key"] = cardEvent.EventKey
			meta["task_id"] = cardEvent.TaskID
			// 可以将选中的值序列化后存入 meta，或 Pipeline 需直接断言 Raw
			text = cardEvent.EventKey // 将 Key 视为指令文本，便于 CommandManager 路由
		} else if msg.Event.FeedbackEvent != nil {
			// 反馈事件
			meta["feedback_id"] = msg.Event.FeedbackEvent.ID
		}
	}

	return botcore.Update{
		ID:       msg.MsgID,
		SenderID: msg.From.UserID,
		ChatID:   msg.ChatID,
		ChatType: msg.ChatType,
		Text:     text,
		Raw:      msg,
		Metadata: meta,
	}, nil
}

// stripLeadingMentions 剥离文本中前置的 @ 提及 token。
// 入参：text 为原始文本内容。
// 返回：移除前置 @ 提及后的文本；若无可剥离内容，则返回原文本。
func stripLeadingMentions(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return text
	}
	if !strings.HasPrefix(trimmed, "@") {
		return text
	}

	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return text
	}

	i := 0
	for i < len(fields) && strings.HasPrefix(fields[i], "@") {
		i++
	}
	if i >= len(fields) {
		return ""
	}
	return strings.Join(fields[i:], " ")
}

// StreamEmitter 将 StreamChunk 转换为企业微信 StreamReply。
type StreamEmitter struct{}

// Encode 将 chunk 降级为 StreamReply 结构体。
func (StreamEmitter) Encode(update botcore.Update, streamID string, chunk botcore.StreamChunk) (interface{}, error) {
	reply := BuildStreamReply(streamID, chunk.Content, chunk.IsFinal)
	return reply, nil
}
