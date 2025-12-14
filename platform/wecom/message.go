package wecom

import (
	"encoding/json"
)

// Message 表示企业微信回调的通用消息结构。
type Message struct {
	MsgID       string             `json:"msgid"`                 // 企业微信消息唯一标识
	CreateTime  int64              `json:"create_time,omitempty"` // 消息创建时间
	AIBotID     string             `json:"aibotid"`               // 机器人 ID
	ChatID      string             `json:"chatid"`                // 群或私聊会话 ID
	ChatType    string             `json:"chattype"`              // chat 类型（single/chatroom）
	From        MessageSender      `json:"from"`                  // 触发者信息
	ResponseURL string             `json:"response_url"`          // 异步回复 URL (部分事件有)
	MsgType     string             `json:"msgtype"`               // 消息类型: text, image, voice, file, mixed, stream, event
	Text        *TextPayload       `json:"text,omitempty"`
	Image       *ImagePayload      `json:"image,omitempty"`
	Voice       *VoicePayload      `json:"voice,omitempty"`
	File        *FilePayload       `json:"file,omitempty"`
	Mixed       *MixedPayload      `json:"mixed,omitempty"`
	Stream      *StreamPayload     `json:"stream,omitempty"`
	Quote       *QuotePayload      `json:"quote,omitempty"`
	Event       *EventPayload      `json:"event,omitempty"`
	Attachment  *AttachmentPayload `json:"attachment,omitempty"` // 某些事件可能带附件
}

// MessageSender 描述消息的触发者。
type MessageSender struct {
	UserID string `json:"userid"`           // 用户 ID
	CorpID string `json:"corpid,omitempty"` // 企业 ID (事件中可能返回)
}

// TextPayload 为文本消息内容。
type TextPayload struct {
	Content string `json:"content"` // 文本内容
}

// ImagePayload 为图片消息内容。
type ImagePayload struct {
	URL    string `json:"url,omitempty"`    // 图片访问地址
	Base64 string `json:"base64,omitempty"` // 流式回复时使用
	MD5    string `json:"md5,omitempty"`    // 流式回复时使用
}

// VoicePayload 为语音消息内容。
type VoicePayload struct {
	Content string `json:"content"` // 语音转文本内容
}

// FilePayload 为文件消息内容。
type FilePayload struct {
	URL string `json:"url"` // 文件下载地址
}

// MixedPayload 表示图文混排消息。
type MixedPayload struct {
	Items []MixedItem `json:"msg_item"`
}

// MixedItem 为图文混排中的单个子消息。
type MixedItem struct {
	MsgType string        `json:"msgtype"`
	Text    *TextPayload  `json:"text,omitempty"`
	Image   *ImagePayload `json:"image,omitempty"`
}

// StreamPayload 表达流式消息的会话信息。
type StreamPayload struct {
	ID      string      `json:"id"`
	Finish  bool        `json:"finish,omitempty"`
	Content string      `json:"content,omitempty"`
	MsgItem []MixedItem `json:"msg_item,omitempty"` // 流式结束时支持图文
}

// QuotePayload 引用消息内容。
type QuotePayload struct {
	MsgType string        `json:"msgtype"`
	Text    *TextPayload  `json:"text,omitempty"`
	Image   *ImagePayload `json:"image,omitempty"`
	Mixed   *MixedPayload `json:"mixed,omitempty"`
	Voice   *VoicePayload `json:"voice,omitempty"`
	File    *FilePayload  `json:"file,omitempty"`
}

// EventPayload 事件结构体
type EventPayload struct {
	EventType         string             `json:"eventtype"`
	EnterChat         *struct{}          `json:"enter_chat,omitempty"`
	TemplateCardEvent *TemplateCardEvent `json:"template_card_event,omitempty"`
	FeedbackEvent     *FeedbackEvent     `json:"feedback_event,omitempty"`
}

// TemplateCardEvent 模板卡片事件
type TemplateCardEvent struct {
	CardType      string         `json:"card_type"`                // 模版类型
	EventKey      string         `json:"event_key"`                // 按钮Key
	TaskID        string         `json:"task_id"`                  // 任务ID
	SelectedItems *SelectedItems `json:"selected_items,omitempty"` // 选择结果
}

// SelectedItems 模板卡片选择结果容器
type SelectedItems struct {
	SelectedItem []SelectedItem `json:"selected_item"`
}

// SelectedItem 单个选择项结果
type SelectedItem struct {
	QuestionKey string     `json:"question_key"`
	OptionIDs   *OptionIDs `json:"option_ids,omitempty"`
}

// OptionIDs 选项ID列表
type OptionIDs struct {
	OptionID []string `json:"option_id"`
}

// FeedbackEvent 用户反馈事件
type FeedbackEvent struct {
	ID                   string `json:"id"`                               // 反馈ID
	Type                 int    `json:"type"`                             // 1:准确, 2:不准确, 3:取消
	Content              string `json:"content,omitempty"`                // 反馈内容
	InaccurateReasonList []int  `json:"inaccurate_reason_list,omitempty"` // 负反馈原因
}

// AttachmentPayload 智能应用回调附件
type AttachmentPayload struct {
	CallbackID string `json:"callback_id"`
	Actions    []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
		Type  string `json:"type"`
	} `json:"actions"`
}

// EncryptedRequest 对应企业微信 POST 回调中的加密请求格式。
type EncryptedRequest struct {
	Encrypt string `json:"encrypt"`
}

// EncryptedResponse 表示向企业微信回复的加密数据包。
type EncryptedResponse struct {
	Encrypt      string `json:"encrypt"`
	MsgSignature string `json:"msgsignature"`
	Timestamp    string `json:"timestamp"`
	Nonce        string `json:"nonce"`
}

// StreamReply 用于构造流式消息回复的明文结构。
type StreamReply struct {
	MsgType string          `json:"msgtype"`
	Stream  StreamReplyBody `json:"stream"`
}

// StreamReplyBody 为流式回复中的具体内容。
type StreamReplyBody struct {
	ID      string      `json:"id"`
	Finish  bool        `json:"finish"`
	Content string      `json:"content"`
	MsgItem []MixedItem `json:"msg_item,omitempty"`
}

// TextMessage 被动回复文本消息
type TextMessage struct {
	MsgType string       `json:"msgtype"`
	Text    *TextPayload `json:"text"`
}

// TemplateCardMessage 被动回复模版卡片消息
type TemplateCardMessage struct {
	MsgType      string        `json:"msgtype"`
	TemplateCard *TemplateCard `json:"template_card"`
}

// StreamWithTemplateCardMessage 被动回复流式+模版卡片
type StreamWithTemplateCardMessage struct {
	MsgType      string          `json:"msgtype"`
	Stream       StreamReplyBody `json:"stream"`
	TemplateCard *TemplateCard   `json:"template_card"`
}

// UpdateTemplateCardMessage 更新模版卡片消息
type UpdateTemplateCardMessage struct {
	ResponseType string        `json:"response_type"` // update_template_card
	UserIDs      []string      `json:"userids,omitempty"`
	TemplateCard *TemplateCard `json:"template_card"`
}

// ParseMessage 将明文 JSON 数据解析为 Message。
func ParseMessage(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// BuildStreamReply 根据 streamID 组装流式回复明文。
func BuildStreamReply(streamID, content string, finish bool) StreamReply {
	return StreamReply{
		MsgType: "stream",
		Stream: StreamReplyBody{
			ID:      streamID,
			Finish:  finish,
			Content: content,
		},
	}
}
