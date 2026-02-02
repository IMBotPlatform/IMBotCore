package wecom

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/IMBotPlatform/IMBotCore/pkg/botcore"
)

var (
	// ErrNoResponse 表示业务层请求不进行任何被动回复（HTTP 200 OK 空包）。
	ErrNoResponse = errors.New("no response")
)

// Bot 集成企业微信回调处理与流式响应逻辑。
// Fields:
//   - streamMgr: 管理流式会话生命周期的 StreamManager
//   - crypto: 负责签名校验与加解密的 Crypt
//   - client: 主动回复客户端，负责向 response_url 发送消息
//   - pipeline: 首包触发的业务流水线实现，可为空
type Bot struct {
	streamMgr *StreamManager
	crypto    *Crypt
	client    *http.Client

	pipeline botcore.PipelineInvoker
}

// NewBot 根据给定参数创建 Bot。
// Parameters:
//   - token: 企业微信配置的消息校验 Token
//   - encodingAESKey: 企业微信后台生成的 43 字节 Base64 编码字符串
//   - corpID: 企业 ID，用于校验消息归属
//   - sessionTTL: 流式会话最大存活时间（<=0 时使用 StreamManager 默认值）
//   - streamWaitTimeout: 刷新请求等待流水线片段的最大时长（<=0 时使用 StreamManager 默认值）
//   - pipeline: 首包触发的业务流水线实现，可为 nil
//
// Returns:
//   - *Bot: 成功初始化的 Bot 实例
//   - error: 当加解密上下文初始化失败时返回错误
func NewBot(token, encodingAESKey, corpID string, streamMsgTTL, streamWaitTimeout time.Duration, pipeline botcore.PipelineInvoker) (*Bot, error) {
	// 关键步骤：在构造 Bot 之前先初始化加解密上下文。
	crypto, err := NewCrypt(token, encodingAESKey, corpID)
	if err != nil {
		return nil, err
	}

	return &Bot{
		streamMgr: newStreamManager(streamMsgTTL, streamWaitTimeout),
		crypto:    crypto,
		client: &http.Client{
			Timeout: resolveDuration(0, envBotHTTPTimeout, 10*time.Second),
		},
		pipeline: pipeline,
	}, nil
}

// ServeHTTP 实现 http.Handler 接口，根据请求方法转发至不同处理逻辑。
// Parameters:
//   - w: http.ResponseWriter，用于写回响应
//   - r: *http.Request，请求上下文
func (b *Bot) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 根据请求方法执行对应的回调处理逻辑。
	switch r.Method {
	case http.MethodGet:
		// GET 请求用于企业微信 URL 验证。
		b.handleGet(w, r)
	case http.MethodPost:
		// POST 请求承载业务事件推送。
		b.handlePost(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGet 处理企业微信服务器验证 URL 的场景，需要校验签名并返回解密后的 echostr。
// Parameters:
//   - w: http.ResponseWriter，用于输出验证明文
//   - r: *http.Request，包含企业微信回调参数
func (b *Bot) handleGet(w http.ResponseWriter, r *http.Request) {
	if b == nil || b.crypto == nil {
		http.Error(w, "server misconfigured", http.StatusInternalServerError)
		return
	}

	// 第一步：解析企业微信回调所需的查询参数。
	query := r.URL.Query()
	sig := query.Get("msg_signature")
	ts := query.Get("timestamp")
	nonce := query.Get("nonce")
	echostr := query.Get("echostr")
	if sig == "" || ts == "" || nonce == "" || echostr == "" {
		http.Error(w, "missing parameters", http.StatusBadRequest)
		return
	}

	// 第二步：调用加解密模块完成签名验证与明文解密。
	plain, err := b.crypto.VerifyURL(sig, ts, nonce, echostr)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// 第三步：以纯文本形式响应企业微信平台。
	w.Header().Set("Content-Type", "text/plain; charset=utf-8") // 企业微信要求返回纯文本
	_, _ = w.Write([]byte(plain))
}

// handlePost 处理企业微信推送的业务回调，完成解密、业务响应构造与加密返回。
// Parameters:
//   - w: http.ResponseWriter，用于返回加密响应
//   - r: *http.Request，包含签名参数与加密 JSON 体
//
// 流程图：
//
//	[读取URL参数] -> [校验缺失?] --是--> [400]
//	     |
//	    否
//	     |
//	     v
//	[解析JSON体] -> [解密消息]
//	     |
//	     v
//	[首次?] -> [Bot.initial] / [Bot.refresh]
//	     |
//	     v
//	[JSON序列化响应] -> [写回加密包]
func (b *Bot) handlePost(w http.ResponseWriter, r *http.Request) {
	if b == nil || b.crypto == nil || b.streamMgr == nil {
		http.Error(w, "server misconfigured", http.StatusInternalServerError)
		return
	}

	// 第一步：请求开始前清理过期会话，避免资源堆积。
	b.cleanup() // 每次请求前清理过期会话，防止堆积
	query := r.URL.Query()
	sig := query.Get("msg_signature")
	ts := query.Get("timestamp")
	nonce := query.Get("nonce")
	if sig == "" || ts == "" || nonce == "" {
		http.Error(w, "missing parameters", http.StatusBadRequest)
		return
	}

	// 第二步：解析请求体中的加密 JSON 数据。
	defer r.Body.Close()
	var req EncryptedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Encrypt == "" {
		http.Error(w, "missing encrypt", http.StatusBadRequest)
		return
	}

	// 第三步：解密业务消息，进入业务处理阶段。
	msg, err := b.crypto.DecryptMessage(sig, ts, nonce, req)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	// 关键步骤：反馈事件仅支持空包回复，直接返回 200，避免进入 pipeline。
	if msg.MsgType == "event" && msg.Event != nil && (msg.Event.FeedbackEvent != nil || msg.Event.EventType == "feedback_event") {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 第四步：区分首次或刷新场景，由 Bot 内部流式逻辑产出响应体。
	var resp EncryptedResponse
	if msg.MsgType == "stream" {
		resp, err = b.refresh(msg, ts, nonce) // 流式刷新请求
	} else {
		resp, err = b.initial(msg, ts, nonce) // 首包或非流式请求
	}

	// 特殊处理：业务层明确要求不回复
	if errors.Is(err, ErrNoResponse) {
		w.WriteHeader(http.StatusOK)
		return
	}

	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// 第五步：序列化加密响应并回写给企业微信平台。
	data, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8") // 企业微信要求 JSON 返回
	_, _ = w.Write(data)
}

// initial 处理首次回调，创建流式会话并触发业务流水线。
// Parameters:
//   - msg: 企业微信解密后的消息
//   - timestamp: 调用方传入的时间戳字符串
//   - nonce: 请求参数中的随机串
//
// Returns:
//   - EncryptedResponse: 首包响应
//   - error: 加密失败时返回
//
// 流程图：
//
//	[创建/复用会话] -> [触发流水线] -> [后台消费输出]
//	                     |
//	                     v
//	              [返回空Stream ACK]
func (b *Bot) initial(msg *Message, timestamp, nonce string) (EncryptedResponse, error) {
	firstSnapshot, err := b.BuildFirstSnapshot(msg)
	if err != nil {
		return EncryptedResponse{}, err
	}

	// 第一步：创建或复用流式会话。
	stream, isNew := b.streamMgr.createOrGet(msg)
	firstSnapshot.ID = stream.StreamID // 关键步骤：企业微信以 streamID 标识流式会话，这里统一写入快照 ID。
	b.streamMgr.setFirstSnapshot(stream.StreamID, firstSnapshot)

	// 关键步骤：首包只负责触发流水线并返回空 ACK，内容由 refresh 拉取。
	if isNew && b.pipeline != nil {
		outCh := b.pipeline.Trigger(firstSnapshot)
		if outCh != nil {
			// 后台消费流水线输出，由 refresh 统一返回内容。
			go b.doPipeline(outCh, stream.StreamID)
		}
	}

	// 第三步：构造首包 ACK（始终为空内容）
	reply, err := b.BuildReply(firstSnapshot, botcore.StreamChunk{Content: "", IsFinal: false})
	if err != nil {
		return EncryptedResponse{}, err
	}
	return b.crypto.EncryptResponse(reply, timestamp, nonce)
}

// refresh 处理企业微信的流式刷新请求。
// Parameters:
//   - msg: 企业微信回调中的流式刷新消息
//   - timestamp: 调用方传入的时间戳字符串
//   - nonce: 随机串
//
// Returns:
//   - EncryptedResponse: 最新片段的加密响应
//   - error: 会话加密失败时返回
//
// 流程图：
//
//	[提取streamID]
//	     |
//	streamID为空?
//	   /    \
//	 是      否
//	 |        |
//
// [返回空终止]  [从Session消费片段]
//
//	|
//
// 取到片段? —否→ [返回空Refresh]
//
//	 |
//	是
//	 |
//
// [标记完成(若final)]
//
//	|
//	v
//
// [加密并返回]
func (b *Bot) refresh(msg *Message, timestamp, nonce string) (EncryptedResponse, error) {
	// 第一步：提取 streamID，判定是否为有效流式刷新请求。
	streamID := ""
	if msg.Stream != nil {
		streamID = msg.Stream.ID
	}
	if streamID == "" {
		// 无效 streamID 直接返回终止包，告知客户端结束。
		reply, err := b.BuildReply(botcore.RequestSnapshot{}, botcore.StreamChunk{Content: "", IsFinal: true})
		if err != nil {
			return EncryptedResponse{}, err
		}
		return b.crypto.EncryptResponse(reply, timestamp, nonce)
	}

	// 第二步：从会话中获取最新累计片段与首包快照。
	firstSnapshot, chunk := b.streamMgr.getLatestChunk(streamID)
	if chunk == nil {
		// 无片段可用，返回保持连接的空包。
		reply, err := b.BuildReply(firstSnapshot, botcore.StreamChunk{Content: "", IsFinal: false})
		if err != nil {
			return EncryptedResponse{}, err
		}
		return b.crypto.EncryptResponse(reply, timestamp, nonce)
	}
	if chunk.IsFinal {
		// 最终片段需标记会话完成，避免资源泄露。
		b.streamMgr.markFinished(streamID)
	}

	// 第四步：将片段封装为流式响应并加密返回。
	reply, err := b.BuildReply(firstSnapshot, *chunk)
	if err != nil {
		return EncryptedResponse{}, err
	}
	return b.crypto.EncryptResponse(reply, timestamp, nonce)
}

// setFinalMessage 在非流式场景下尝试投递最终结果（仅在会话存在时生效）。
// Parameters:
//   - msgID: 企业微信消息 ID
//   - content: 业务最终结果
func (b *Bot) setFinalMessage(msgID, content string) {
	if msgID == "" {
		return
	}
	streamID, ok := b.streamMgr.getStreamIDByMsg(msgID)
	if !ok || streamID == "" {
		return
	}
	chunk := botcore.StreamChunk{Content: content, IsFinal: true}
	b.streamMgr.publish(streamID, chunk)
}

// cleanup 清理过期会话，防止 Session 过度累积。
func (b *Bot) cleanup() {
	if b == nil || b.streamMgr == nil {
		return
	}

	// 委托 StreamManager 移除超时会话。
	b.streamMgr.cleanup()
}

// doPipeline 消费流水线输出并发布到流式会话。
// Parameters:
//   - outCh: 流水线输出通道
//   - streamID: 目标流式会话 ID
//
// 说明：
//   - NoResponse 代表立即结束会话
//   - 若无任何输出，则发送空终止片段避免 refresh 无限轮询
func (b *Bot) doPipeline(outCh <-chan botcore.StreamChunk, streamID string) {
	if outCh == nil {
		return
	}

	published := false
	for chunk := range outCh {
		// 关键步骤：NoResponse 在流式场景中等价于“立即结束”。
		if chunk.Payload == botcore.NoResponse {
			if b.streamMgr.publish(streamID, botcore.StreamChunk{Content: "", IsFinal: true}) {
				published = true
			}
			return
		}
		if chunk.Content == "" && chunk.Payload == nil && !chunk.IsFinal {
			continue
		}

		if b.streamMgr.publish(streamID, chunk) {
			published = true
		}
	}
	if !published {
		// 无输出也要结束会话，避免 refresh 无限轮询。
		b.streamMgr.publish(streamID, botcore.StreamChunk{Content: "", IsFinal: true})
	}
}

// mapWecomChatType 将企业微信 chattype 规范化为内部标准类型。
// Parameters:
//   - raw: 企业微信回调中的原始 chattype 字符串
//
// Returns:
//   - botcore.ChatType: 标准化后的会话类型
func mapWecomChatType(raw string) botcore.ChatType {
	// 关键步骤：统一大小写并剔除空白，避免上游传入非规范值。
	normalized := strings.ToLower(strings.TrimSpace(raw))
	switch normalized {
	case "single":
		return botcore.ChatTypeSingle
	case "group", "chatroom":
		return botcore.ChatTypeChatroom
	default:
		return botcore.ChatType(raw)
	}
}

// BuildFirstSnapshot 构建首包快照。
// Parameters:
//   - raw: 平台原始消息结构
//
// Returns:
//   - botcore.RequestSnapshot: 标准化首包快照
//   - error: 构建失败时返回
func (b *Bot) BuildFirstSnapshot(raw any) (botcore.RequestSnapshot, error) {
	msg, ok := raw.(*Message)
	if !ok || msg == nil {
		return botcore.RequestSnapshot{}, errors.New("invalid wecom message")
	}

	text := ""
	if msg.Text != nil {
		text = msg.Text.Content
	}

	// metadata 约定键：platform/msgtype/response_url/stream_id/event_type/task_id/feedback_* 等。
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
			// 进入会话事件：不再生成隐式指令，由上层根据 event_type 处理。
		} else if msg.Event.TemplateCardEvent != nil {
			// 模板卡片事件
			cardEvent := msg.Event.TemplateCardEvent
			if cardEvent.EventKey != "" {
				// 将 Key 视为指令文本，便于 CommandManager 路由。
				text = cardEvent.EventKey
			}
			if cardEvent.TaskID != "" {
				meta["task_id"] = cardEvent.TaskID
			}
		} else if msg.Event.FeedbackEvent != nil {
			// 反馈事件
			feedback := msg.Event.FeedbackEvent
			// 关键步骤：将反馈事件字段写入 metadata，供业务层使用。
			meta["feedback_id"] = feedback.ID
			meta["feedback_type"] = strconv.Itoa(feedback.Type)
			if feedback.Content != "" {
				meta["feedback_content"] = feedback.Content
			}
		}
	}

	attachments := make([]botcore.Attachment, 0)
	addAttachment := func(attType botcore.AttachmentType, url string) {
		if strings.TrimSpace(url) == "" {
			return
		}
		attachments = append(attachments, botcore.Attachment{Type: attType, URL: url})
	}

	// 关键步骤：从消息体中抽取图片/文件附件。
	switch msg.MsgType {
	case "image":
		if msg.Image != nil {
			addAttachment(botcore.AttachmentTypeImage, msg.Image.URL)
		}
	case "file":
		if msg.File != nil {
			addAttachment(botcore.AttachmentTypeFile, msg.File.URL)
		}
	case "mixed":
		if msg.Mixed != nil {
			for _, item := range msg.Mixed.Items {
				if item.MsgType == "image" && item.Image != nil {
					addAttachment(botcore.AttachmentTypeImage, item.Image.URL)
				}
			}
		}
	}

	// 引用消息中的附件也纳入统一列表。
	if msg.Quote != nil {
		switch msg.Quote.MsgType {
		case "image":
			if msg.Quote.Image != nil {
				addAttachment(botcore.AttachmentTypeImage, msg.Quote.Image.URL)
			}
		case "file":
			if msg.Quote.File != nil {
				addAttachment(botcore.AttachmentTypeFile, msg.Quote.File.URL)
			}
		case "mixed":
			if msg.Quote.Mixed != nil {
				for _, item := range msg.Quote.Mixed.Items {
					if item.MsgType == "image" && item.Image != nil {
						addAttachment(botcore.AttachmentTypeImage, item.Image.URL)
					}
				}
			}
		}
	}

	return botcore.RequestSnapshot{
		ID:          "",
		SenderID:    msg.From.UserID,
		ChatID:      msg.ChatID,
		ChatType:    mapWecomChatType(msg.ChatType),
		Text:        text,
		Attachments: attachments,
		Raw:         msg,
		ResponseURL: msg.ResponseURL,
		Metadata:    meta,
	}, nil
}

// Send 向指定的 response_url 发送主动回复消息。
// 对应文档：7_加解密说明.md - 如何主动回复消息
// 注意：response_url 有效期为 1 小时，且每个 url 仅可调用一次。
// Parameters:
//   - responseURL: 企业微信回调中提供的 response_url
//   - msg: 待发送的消息负载（会被序列化为 JSON）
//
// Returns:
//   - error: 发送失败或序列化失败时返回
func (b *Bot) Send(responseURL string, msg any) error {
	if responseURL == "" {
		return fmt.Errorf("response_url is empty")
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, responseURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("wecom api error: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	// 企业微信 API 通常返回 JSON，包含 errcode。这里简单检查 status code。
	// 如果需要更严格的错误检查，可以解析 respBody 中的 errcode。
	return nil
}

// MarkdownMessage 主动回复 Markdown 消息结构。
type MarkdownMessage struct {
	MsgType  string          `json:"msgtype"` // markdown
	Markdown MarkdownPayload `json:"markdown"`
}

// MarkdownPayload 表示 Markdown 消息体内容。
type MarkdownPayload struct {
	Content  string        `json:"content"`            // Markdown 文本内容
	Feedback *FeedbackInfo `json:"feedback,omitempty"` // 可选反馈信息
}

// SendMarkdown 发送 Markdown 消息。
// Parameters:
//   - responseURL: 企业微信回调中提供的 response_url
//   - content: Markdown 文本内容
//
// Returns:
//   - error: 发送失败时返回
func (b *Bot) SendMarkdown(responseURL, content string) error {
	msg := MarkdownMessage{
		MsgType: "markdown",
		Markdown: MarkdownPayload{
			Content: content,
		},
	}
	return b.Send(responseURL, msg)
}

// SendTemplateCard 发送模板卡片消息。
// Parameters:
//   - responseURL: 企业微信回调中提供的 response_url
//   - card: 模板卡片负载（需为 *TemplateCard）
//
// Returns:
//   - error: 发送失败或类型不匹配时返回
func (b *Bot) SendTemplateCard(responseURL string, card any) error {
	typedCard, ok := card.(*TemplateCard)
	if !ok {
		return fmt.Errorf("invalid card type: expected *TemplateCard, got %T", card)
	}
	msg := TemplateCardMessage{
		MsgType:      "template_card",
		TemplateCard: typedCard,
	}
	return b.Send(responseURL, msg)
}

// BuildReply 将流式片段编码为平台响应。
// Parameters:
//   - snapshot: 首包快照
//   - chunk: 流式片段
//
// Returns:
//   - any: 平台响应负载
//   - error: 编码失败时返回
func (b *Bot) BuildReply(firstSnapshot botcore.RequestSnapshot, chunk botcore.StreamChunk) (any, error) {
	// 优先处理携带的非流式 Payload
	if chunk.Payload != nil {
		return chunk.Payload, nil
	}

	return buildStreamReply(firstSnapshot.ID, chunk.Content, chunk.IsFinal), nil
}
