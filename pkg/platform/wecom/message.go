package wecom

import "encoding/json"

// ==================================== Request ====================================

// Message 表示企业微信回调的通用消息结构。
type Message struct {
	MsgID       string             `json:"msgid"`                 // 企业微信消息唯一标识
	CreateTime  int64              `json:"create_time,omitempty"` // 消息创建时间
	AIBotID     string             `json:"aibotid"`               // 机器人 ID
	ChatID      string             `json:"chatid"`                // 群或私聊会话 ID
	ChatType    string             `json:"chattype"`              // chat 类型（single/group）
	From        MessageSender      `json:"from"`                  // 触发者信息
	ResponseURL string             `json:"response_url"`          // 异步回复 URL (部分事件有)
	MsgType     string             `json:"msgtype"`               // 消息类型: text, image, voice, file, mixed, stream, event
	Text        *TextPayload       `json:"text,omitempty"`        // 文本消息内容（MsgType=text）
	Image       *ImagePayload      `json:"image,omitempty"`       // 图片消息内容（MsgType=image）
	Voice       *VoicePayload      `json:"voice,omitempty"`       // 语音消息内容（MsgType=voice）
	File        *FilePayload       `json:"file,omitempty"`        // 文件消息内容（MsgType=file）
	Mixed       *MixedPayload      `json:"mixed,omitempty"`       // 图文混排内容（MsgType=mixed）
	Stream      *StreamPayload     `json:"stream,omitempty"`      // 流式消息内容（MsgType=stream）
	Quote       *QuotePayload      `json:"quote,omitempty"`       // 引用消息内容（MsgType=quote）
	Event       *EventPayload      `json:"event,omitempty"`       // 事件消息内容（MsgType=event）
	Attachment  *AttachmentPayload `json:"attachment,omitempty"`  // 某些事件可能带附件
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
	Items []MixedItem `json:"msg_item"` // 图文混排子消息列表
}

// MixedItem 为图文混排中的单个子消息。
type MixedItem struct {
	MsgType string        `json:"msgtype"`         // 子消息类型
	Text    *TextPayload  `json:"text,omitempty"`  // 文本子消息
	Image   *ImagePayload `json:"image,omitempty"` // 图片子消息
}

// StreamPayload 表达流式消息的会话信息。
type StreamPayload struct {
	ID      string      `json:"id"`                 // 流式会话 ID
	Finish  bool        `json:"finish,omitempty"`   // 是否结束
	Content string      `json:"content,omitempty"`  // 当前累计内容
	MsgItem []MixedItem `json:"msg_item,omitempty"` // 流式结束时支持图文
}

// QuotePayload 引用消息内容。
type QuotePayload struct {
	MsgType string        `json:"msgtype"`         // 引用消息类型
	Text    *TextPayload  `json:"text,omitempty"`  // 引用文本
	Image   *ImagePayload `json:"image,omitempty"` // 引用图片
	Mixed   *MixedPayload `json:"mixed,omitempty"` // 引用图文混排
	Voice   *VoicePayload `json:"voice,omitempty"` // 引用语音
	File    *FilePayload  `json:"file,omitempty"`  // 引用文件
}

// EventPayload 事件结构体
type EventPayload struct {
	EventType         string             `json:"eventtype"`                     // 事件类型标识
	EnterChat         *struct{}          `json:"enter_chat,omitempty"`          // 进入会话事件
	TemplateCardEvent *TemplateCardEvent `json:"template_card_event,omitempty"` // 模板卡片事件
	FeedbackEvent     *FeedbackEvent     `json:"feedback_event,omitempty"`      // 反馈事件
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
	SelectedItem []SelectedItem `json:"selected_item"` // 选择结果列表
}

// SelectedItem 单个选择项结果
type SelectedItem struct {
	QuestionKey string     `json:"question_key"`         // 题目 key
	OptionIDs   *OptionIDs `json:"option_ids,omitempty"` // 选中的选项
}

// OptionIDs 选项ID列表
type OptionIDs struct {
	OptionID []string `json:"option_id"` // 选项 ID 列表
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
	CallbackID string `json:"callback_id"` // 回调 ID
	Actions    []struct {
		Name  string `json:"name"`  // 动作名称
		Value string `json:"value"` // 动作值
		Type  string `json:"type"`  // 动作类型
	} `json:"actions"`
}

// EncryptedRequest 对应企业微信 POST 回调中的加密请求格式。
type EncryptedRequest struct {
	Encrypt string `json:"encrypt"` // 企业微信回调中的加密字符串
}

// parseMessage 将明文 JSON 数据解析为 Message。
// Parameters:
//   - data: 明文字节数组
//
// Returns:
//   - *Message: 解码后的消息结构
//   - error: JSON 反序列化失败时返回
func parseMessage(data []byte) (*Message, error) {
	var msg Message
	// 关键步骤：将 JSON 明文解析为业务消息结构。
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// ==================================== Response ====================================

// EncryptedResponse 表示向企业微信回复的加密数据包。
type EncryptedResponse struct {
	Encrypt      string `json:"encrypt"`      // Base64 编码的密文
	MsgSignature string `json:"msgsignature"` // 签名，用于企业微信校验
	Timestamp    string `json:"timestamp"`    // 时间戳
	Nonce        string `json:"nonce"`        // 随机串
}

// StreamReply 用于构造流式消息回复的明文结构。
type StreamReply struct {
	MsgType string          `json:"msgtype"` // 固定为 stream
	Stream  StreamReplyBody `json:"stream"`  // 流式消息体
}

// StreamReplyBody 为流式回复中的具体内容。
type StreamReplyBody struct {
	ID       string        `json:"id"`                 // 流式会话 ID
	Finish   bool          `json:"finish"`             // 是否结束
	Content  string        `json:"content"`            // 累计内容
	MsgItem  []MixedItem   `json:"msg_item,omitempty"` // 结束时可携带图文
	Feedback *FeedbackInfo `json:"feedback,omitempty"` // 反馈信息（流式回复可选）
}

// TextMessage 被动回复文本消息
type TextMessage struct {
	MsgType string       `json:"msgtype"` // 固定为 text
	Text    *TextPayload `json:"text"`    // 文本内容
}

// TemplateCardMessage 被动回复模版卡片消息
type TemplateCardMessage struct {
	MsgType      string        `json:"msgtype"`       // 固定为 template_card
	TemplateCard *TemplateCard `json:"template_card"` // 模板卡片体
}

// StreamWithTemplateCardMessage 被动回复流式+模版卡片
type StreamWithTemplateCardMessage struct {
	MsgType      string          `json:"msgtype"`       // 固定为 stream
	Stream       StreamReplyBody `json:"stream"`        // 流式消息体
	TemplateCard *TemplateCard   `json:"template_card"` // 模板卡片体
}

// UpdateTemplateCardMessage 更新模版卡片消息
type UpdateTemplateCardMessage struct {
	ResponseType string        `json:"response_type"`     // 固定为 update_template_card
	UserIDs      []string      `json:"userids,omitempty"` // 指定接收用户
	TemplateCard *TemplateCard `json:"template_card"`     // 模板卡片体
}

// buildStreamReply 根据 streamID 组装流式回复明文。
// Parameters:
//   - streamID: 流式会话 ID
//   - content: 当前累计内容
//   - finish: 是否结束
//
// Returns:
//   - StreamReply: 组装后的流式回复体
func buildStreamReply(streamID, content string, finish bool) StreamReply {
	return StreamReply{
		MsgType: "stream",
		Stream: StreamReplyBody{
			ID:      streamID,
			Finish:  finish,
			Content: content,
		},
	}
}
