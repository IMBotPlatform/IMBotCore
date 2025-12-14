package wecom

import (
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/IMBotPlatform/IMBotCore/botcore"
)

var (
	// ErrNoResponse 表示业务层请求不进行任何被动回复（HTTP 200 OK 空包）。
	ErrNoResponse = errors.New("no response")
)

// Bot 集成企业微信回调处理与流式响应逻辑。
// Fields:
//   - Sessions: 管理流式会话生命周期的 SessionManager
//   - Crypto: 负责签名校验与加解密的 Crypt
//   - Pipeline: 首包触发的业务流水线实现，可为空
//   - Timeout: 刷新请求等待流水线片段的最大时长
type Bot struct {
	Sessions *SessionManager
	Crypto   *Crypt
	Pipeline botcore.PipelineInvoker
	Adapter  botcore.Adapter
	Emitter  botcore.Emitter
	Timeout  time.Duration

	fallback sync.Map // msgid -> botcore.StreamChunk，用于记录未及时下发的最终片段。
}

// BotOption 用于定制 Bot 行为。
type BotOption func(*Bot)

// WithAdapter 自定义消息标准化适配器。
func WithAdapter(adapter botcore.Adapter) BotOption {
	return func(b *Bot) {
		b.Adapter = adapter
	}
}

// WithEmitter 覆盖默认的流式响应构造器。
func WithEmitter(emitter botcore.Emitter) BotOption {
	return func(b *Bot) {
		b.Emitter = emitter
	}
}

// NewBot 根据给定参数创建 Bot。
// Parameters:
//   - crypto: 企业微信加解密上下文，不能为空
//   - sessionTTL: 会话最大存活时间（<=0 时使用 SessionManager 默认值）
//   - timeout: 刷新请求等待流水线片段的最大时长（<=0 时在 Refresh 内回退默认值）
//   - pipeline: 首包触发的业务流水线实现，可为 nil
//
// Returns:
//   - *Bot: 成功初始化的 Bot 实例
//   - error: 当 crypto 为空时返回错误
func NewBot(crypto *Crypt, sessionTTL, timeout time.Duration, pipeline botcore.PipelineInvoker, opts ...BotOption) (*Bot, error) {
	if crypto == nil {
		return nil, errors.New("crypto is required")
	}

	sessions := NewSessionManager(sessionTTL)
	bot := &Bot{
		Sessions: sessions,
		Crypto:   crypto,
		Pipeline: pipeline,
		Timeout:  timeout,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(bot)
		}
	}
	if bot.Adapter == nil {
		bot.Adapter = MessageAdapter{}
	}
	if bot.Emitter == nil {
		bot.Emitter = StreamEmitter{}
	}
	return bot, nil
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
	if b == nil || b.Crypto == nil {
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
	plain, err := b.Crypto.VerifyURL(sig, ts, nonce, echostr)
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
//	[首次?] -> [Bot.Initial] / [Bot.Refresh]
//	     |
//	     v
//	[JSON序列化响应] -> [写回加密包]
func (b *Bot) handlePost(w http.ResponseWriter, r *http.Request) {
	if b == nil || b.Crypto == nil || b.Sessions == nil {
		http.Error(w, "server misconfigured", http.StatusInternalServerError)
		return
	}

	// 第一步：请求开始前清理过期会话，避免资源堆积。
	b.Cleanup() // 每次请求前清理过期会话，防止堆积
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
	msg, err := b.Crypto.DecryptMessage(sig, ts, nonce, req)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// 第四步：区分首次或刷新场景，由 Bot 内部流式逻辑产出响应体。
	var resp EncryptedResponse
	if msg.MsgType == "stream" {
		resp, err = b.Refresh(msg, ts, nonce) // 流式刷新请求
	} else {
		resp, err = b.Initial(msg, ts, nonce) // 首包或非流式请求
	}

	// 特殊处理：业务层明确要求不回复
	if err == ErrNoResponse {
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

// Initial 处理首次回调，创建流式会话并触发业务流水线。
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
//	[创建/复用会话] -> [触发流水线] -> [等待首帧] -> [有数据?] --是--> [Payload?] -> [直接回复]
//	                                         |          \
//	                                         |           -> [Stream?] -> [首包携带内容] -> [后台消费剩余]
//	                                         |
//	                                         -> [超时/无数据] -> [返回空StreamStart] -> [后台消费]
func (b *Bot) Initial(msg *Message, timestamp, nonce string) (EncryptedResponse, error) {
	if b.Adapter == nil {
		return EncryptedResponse{}, errors.New("adapter not configured")
	}
	update, err := b.Adapter.Normalize(msg)
	if err != nil {
		return EncryptedResponse{}, err
	}

	// 第一步：创建或复用流式会话。
	session, isNew := b.Sessions.CreateOrGet(msg)
	b.Sessions.SetUpdate(session.StreamID, update)

	// 默认首包（空流式开始）
	initialChunk := botcore.StreamChunk{Content: "", IsFinal: false}

	if isNew && b.Pipeline != nil {
		outCh := b.Pipeline.Trigger(update, session.StreamID)
		if outCh != nil {
			// 尝试同步等待第一帧，优化首字体验或支持同步响应
			select {
			case chunk, ok := <-outCh:
				if ok {
					// Case 0: 静默信号
					if chunk.Payload == botcore.NoResponse {
						b.Sessions.MarkFinished(session.StreamID)
						return EncryptedResponse{}, ErrNoResponse
					}

					if chunk.Payload != nil {
						// Case 1: 非流式 Payload（如 TemplateCard），直接一次性响应
						// 这种情况下通常意味着对话结束，或者是独立的事件响应
						if chunk.IsFinal {
							b.Sessions.MarkFinished(session.StreamID)
						}
						// 如果还有后续数据，需启动后台消费（虽然对于 Payload 响应通常不应有后续）
						go b.consumePipeline(outCh, msg.MsgID, session.StreamID)

						reply, err := b.buildReply(update, session.StreamID, chunk)
						if err != nil {
							return EncryptedResponse{}, err
						}
						return b.Crypto.EncryptResponse(reply, timestamp, nonce)
					}

					// Case 2: 流式内容，首包携带数据
					// 注意：这里不将 chunk push 到 session，因为我们直接在 Initial 返回了
					// 后续的 Refresh 将消费后续的帧
					b.Sessions.Accumulate(session.StreamID, chunk.Content)
					initialChunk = chunk
					if chunk.IsFinal {
						b.Sessions.MarkFinished(session.StreamID)
					}
					// 启动后台消费剩余帧
					go b.consumePipeline(outCh, msg.MsgID, session.StreamID)
				} else {
					// Channel closed immediately
					b.Sessions.MarkFinished(session.StreamID)
					initialChunk = botcore.StreamChunk{Content: "", IsFinal: true}
				}
			case <-time.After(200 * time.Millisecond):
				// Case 3: 超时未产出，转后台消费，Initial 返回空包
				// 注意：这里存在一个微小的竞态条件，如果此时 outCh 刚好有数据，
				// 可能会被后台协程抢占。但由于 Initial 已经决定返回空包，
				// 后续数据通过 Refresh 获取也是符合预期的。
				go b.consumePipeline(outCh, msg.MsgID, session.StreamID)
			}
		}
	}

	// 第三步：构造首包（可能是空的，也可能包含第一帧文本）
	reply, err := b.buildReply(update, session.StreamID, initialChunk)
	if err != nil {
		return EncryptedResponse{}, err
	}
	return b.Crypto.EncryptResponse(reply, timestamp, nonce)
}

// Refresh 处理企业微信的流式刷新请求。
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
// 取到片段? —否→ [fallback命中?] —否→ [返回空Refresh]
//
//	 |                        |
//	是                        是
//	 |                        |
//
// [标记完成(若final)]         [使用fallback片段]
//
//	|
//	v
//
// [加密并返回]
func (b *Bot) Refresh(msg *Message, timestamp, nonce string) (EncryptedResponse, error) {
	// 第一步：提取 streamID，判定是否为有效流式刷新请求。
	streamID := ""
	if msg.Stream != nil {
		streamID = msg.Stream.ID
	}
	if streamID == "" {
		// 无效 streamID 直接返回终止包，告知客户端结束。
		reply, err := b.buildReply(botcore.Update{}, "", botcore.StreamChunk{Content: "", IsFinal: true})
		if err != nil {
			return EncryptedResponse{}, err
		}
		return b.Crypto.EncryptResponse(reply, timestamp, nonce)
	}

	// 第二步：设置等待窗口，从会话中阻塞消费片段。
	timeout := b.Timeout
	if timeout <= 0 {
		timeout = 500 * time.Millisecond
	}
	chunk := b.Sessions.Consume(streamID, timeout) // 阻塞等待流水线产出
	if chunk == nil {
		// 第三步：消费失败时尝试回退到 fallback，保证终止片段可达。
		if msg.MsgID != "" {
			if cached, ok := b.fallback.LoadAndDelete(msg.MsgID); ok {
				if stored, ok := cached.(botcore.StreamChunk); ok {
					chunk = &stored
				}
			}
		}
	}
	if chunk == nil {
		// 无片段可用，返回保持连接的空包。
		update := b.Sessions.GetUpdate(streamID)
		reply, err := b.buildReply(update, streamID, botcore.StreamChunk{Content: "", IsFinal: false})
		if err != nil {
			return EncryptedResponse{}, err
		}
		return b.Crypto.EncryptResponse(reply, timestamp, nonce)
	}
	if chunk.IsFinal {
		// 最终片段需标记会话完成，避免资源泄露。
		b.Sessions.MarkFinished(streamID)
	}

	// 第四步：将片段封装为流式响应并加密返回。
	update := b.Sessions.GetUpdate(streamID)
	reply, err := b.buildReply(update, streamID, *chunk)
	if err != nil {
		return EncryptedResponse{}, err
	}
	return b.Crypto.EncryptResponse(reply, timestamp, nonce)
}

// PushStreamChunk 将流水线片段推送到对应 stream 会话。
// Parameters:
//   - msgID: 企业微信消息 ID，用于定位会话
//   - content: 流水线产生的文本
//   - isFinal: 是否为最终片段
//
// Returns:
//   - bool: 表示是否成功投递到会话或缓存
//
// 流程图：
//
//	[查找streamID]
//	     |
//	找到?  --否--> [final?] --是--> [缓存fallback] --否--> [返回false]
//	     |
//	    是
//	     |
//	[构造chunk并尝试Publish] --失败--> [如final则缓存fallback]
//	     |
//	    成功
//	     |
//	[final?] --是--> [标记会话完成]
//	     |
//	    返回true
func (b *Bot) pushStreamChunk(streamID, msgID string, chunk botcore.StreamChunk) bool {
	target := streamID
	if target == "" && msgID != "" {
		if located, ok := b.Sessions.GetStreamIDByMsg(msgID); ok {
			target = located
		}
	}
	if target == "" {
		return b.cacheFallback(msgID, chunk)
	}
	if !b.Sessions.Publish(target, chunk) {
		return b.cacheFallback(msgID, chunk)
	}
	if chunk.IsFinal {
		b.Sessions.MarkFinished(target)
	}
	return true
}

func (b *Bot) cacheFallback(msgID string, chunk botcore.StreamChunk) bool {
	// 仅缓存终结片段：当刷新请求找不到会话或 Publish 失败时，
	// 通过 msgID 兜底返回最终结果，保证企业微信侧能收到结束信号。
	if chunk.IsFinal && msgID != "" {
		b.fallback.Store(msgID, chunk)
	}
	return false
}

// PushStreamChunk 兼容旧接口，便于在流水线外部直接推送片段。
func (b *Bot) PushStreamChunk(msgID, content string, isFinal bool) bool {
	return b.pushStreamChunk("", msgID, botcore.StreamChunk{Content: content, IsFinal: isFinal})
}

// SetFinalMessage 在非流式场景下缓存最终结果以备刷新，找不到会话时写入 fallback。
// Parameters:
//   - msgID: 企业微信消息 ID
//   - content: 业务最终结果
func (b *Bot) SetFinalMessage(msgID, content string) {
	chunk := botcore.StreamChunk{Content: content, IsFinal: true}
	b.pushStreamChunk("", msgID, chunk)
}

// Cleanup 清理过期会话，防止 Session 过度累积。
func (b *Bot) Cleanup() {
	if b == nil || b.Sessions == nil {
		return
	}

	// 委托 SessionManager 移除超时会话。
	b.Sessions.Cleanup()
}

func (b *Bot) consumePipeline(outCh <-chan botcore.StreamChunk, msgID, streamID string) {
	if outCh == nil {
		return
	}
	for chunk := range outCh {
		if chunk.Content == "" && chunk.Payload == nil && !chunk.IsFinal {
			continue
		}
		b.pushStreamChunk(streamID, msgID, chunk)
	}
}

func (b *Bot) buildReply(update botcore.Update, streamID string, chunk botcore.StreamChunk) (interface{}, error) {
	// 优先处理携带的非流式 Payload
	if chunk.Payload != nil {
		return chunk.Payload, nil
	}

	if b == nil || b.Emitter == nil {
		return BuildStreamReply(streamID, chunk.Content, chunk.IsFinal), nil
	}
	payload, err := b.Emitter.Encode(update, streamID, chunk)
	if err != nil {
		return nil, err
	}
	return payload, nil
}
