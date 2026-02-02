package wecom

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/IMBotPlatform/IMBotCore/pkg/botcore"
)

// Stream 表示一次流式会话的上下文。
type Stream struct {
	RequestSnapshot botcore.RequestSnapshot // 标准化事件上下文

	StreamID    string    // 流式会话唯一标识
	MsgID       string    // 对应企业微信消息 ID
	ChatID      string    // 会话所属聊天 ID
	UserID      string    // 发起用户 ID
	ResponseURL string    // 主动回复 URL（部分事件返回）
	CreatedAt   time.Time // 创建时间
	LastAccess  time.Time // 最近访问时间

	Finished  bool                     // 会话是否已完成
	queue     chan botcore.StreamChunk // 缓冲队列，存储待下发的流式片段
	LastChunk *botcore.StreamChunk     // 最近一次发送的片段，用于超时兜底
	mu        sync.Mutex               // 保护会话内状态的互斥锁
}

// StreamManager 管理流式会话的生命周期。
type StreamManager struct {
	mu       sync.RWMutex       // 读写锁，保护内部映射
	streams  map[string]*Stream // streamID -> Stream 映射
	msgIndex map[string]string  // msgID -> streamID 索引
	ttl      time.Duration      // 会话超时时长
	timeout  time.Duration      // 消费流式片段的等待时长
}

// newStreamManager 创建 StreamManager。
// Parameters:
//   - ttl: 会话最长存活时间，非正值时回退为 1 分钟
//   - timeout: 消费流式片段等待时长，非正值时回退为 500ms
//
// Returns:
//   - *StreamManager: 管理会话的实例
func newStreamManager(ttl, timeout time.Duration) *StreamManager {
	ttl = resolveDuration(ttl, envBotStreamTTL, time.Minute)
	timeout = resolveDuration(timeout, envBotStreamWaitTimeout, 500*time.Millisecond)

	// 初始化会话管理器，建立基础映射结构。
	return &StreamManager{
		streams:  make(map[string]*Stream),
		msgIndex: make(map[string]string),
		ttl:      ttl,
		timeout:  timeout,
	}
}

// createOrGet 根据消息创建或返回既有会话。
// Parameters:
//   - msg: 企业微信消息体
//
// Returns:
//   - *Stream: 匹配或新建的会话
//   - bool: 是否创建了新会话
//
// 流程图：
//
//	[收到Message]
//	     |
//	有msgID?
//	   /   \
//	 是     否
//	 |      |
//
// [查msgIndex]
//
//	  |
//	找到?
//	 / \
//	是  否
//	|    |
//
// [返回旧会话]   [生成新streamID]
//
//	       |
//	[初始化Stream并索引]
//	       |
//	[返回新会话+isNew]
func (m *StreamManager) createOrGet(msg *Message) (*Stream, bool) {
	var existing *Stream
	if msg.MsgID != "" {
		// 尝试依据消息 ID 命中既有的流式会话。
		if streamID, ok := m.getStreamIDByMsg(msg.MsgID); ok {
			existing = m.getStream(streamID)
		}
	}
	if existing != nil {
		// 若命中已有会话，则刷新访问时间并直接返回复用。
		existing.touch()
		return existing, false
	}

	// 未命中时创建全新的会话上下文。
	streamID := generateStreamID()
	stream := &Stream{
		StreamID:    streamID,
		MsgID:       msg.MsgID,
		ChatID:      msg.ChatID,
		UserID:      msg.From.UserID,
		ResponseURL: msg.ResponseURL,
		CreatedAt:   time.Now(),
		LastAccess:  time.Now(),
		queue:       make(chan botcore.StreamChunk, 16), // 固定容量缓冲，避免无界增长
	}
	m.mu.Lock()
	m.streams[streamID] = stream
	if msg.MsgID != "" {
		m.msgIndex[msg.MsgID] = streamID
	}
	m.mu.Unlock()

	return stream, true
}

// publish 向指定会话写入流式片段，队列满时会阻塞等待消费后写入。
// Parameters:
//   - streamID: 会话标识
//   - chunk: 待推送的流式数据
//
// Returns:
//   - bool: 成功写入返回 true
func (m *StreamManager) publish(streamID string, chunk botcore.StreamChunk) bool {
	stream := m.getStream(streamID)
	if stream == nil {
		return false
	}

	// 加锁更新会话活跃状态与最后一次片段。
	stream.mu.Lock()
	stream.LastAccess = time.Now()
	fullChunk := chunk
	// 企业微信要求 content 为“最新完整内容”，因此这里累积全文后再入队。
	// 注意：若携带 Payload，视为非文本回复，清空累计内容。
	if chunk.Payload == nil && stream.LastChunk != nil {
		fullChunk.Content = stream.LastChunk.Content + chunk.Content
	} else if chunk.Payload != nil {
		fullChunk.Content = ""
	}
	stream.LastChunk = &fullChunk
	finished := fullChunk.IsFinal
	stream.mu.Unlock()

	// 尝试无阻塞写入队列，队列满则等待消费后写入。
	select {
	case stream.queue <- fullChunk:
	default:
		stream.queue <- fullChunk
	}
	if finished {
		// 终结片段需立即标记会话完成。
		stream.setFinished()
	}

	return true
}

// getLatestChunk 获取指定 streamID 的首包快照与最新累计片段。
// Parameters:
//   - streamID: 会话标识
//
// Returns:
//   - botcore.RequestSnapshot: 首包快照（若未绑定则仅填充 ID）
//   - *botcore.StreamChunk: 最新累计片段，可能为 nil
func (m *StreamManager) getLatestChunk(streamID string) (botcore.RequestSnapshot, *botcore.StreamChunk) {
	if streamID == "" {
		return botcore.RequestSnapshot{}, nil
	}

	// 关键步骤：快照 ID 始终与 streamID 对齐，确保回复具备完整标识。
	snapshot := m.getFirstSnapshot(streamID)
	if snapshot.ID == "" {
		snapshot.ID = streamID
	}

	stream := m.getStream(streamID)
	if stream == nil {
		return snapshot, nil
	}
	timeout := m.timeout
	if timeout <= 0 {
		timeout = resolveDuration(0, envBotStreamWaitTimeout, 500*time.Millisecond)
	}

	// 初始化超时控制器，避免无限阻塞消费。
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	// 访问会话时刷新最后活跃时间，保持会话存活。
	stream.touch()

	select {
	case firstChunk := <-stream.queue:
		// 只保留队列中最新的片段（它已经是“完整内容”的快照）。
		latestChunk := firstChunk
		finalSeen := firstChunk.IsFinal
		drained := false
		for !drained {
			select {
			case nextChunk := <-stream.queue:
				latestChunk = nextChunk
				if nextChunk.IsFinal {
					finalSeen = true
				}
			default:
				drained = true
			}
		}
		if finalSeen {
			latestChunk.IsFinal = true
		}

		// 更新状态后返回最新片段。
		stream.mu.Lock()
		stream.LastAccess = time.Now()
		stream.LastChunk = &latestChunk
		if latestChunk.IsFinal {
			stream.Finished = true
		}
		stream.mu.Unlock()
		return snapshot, &latestChunk
	case <-timer.C:
		// 超时未获取到片段时，回退到缓存的最后一次片段。
		stream.mu.Lock()
		stream.LastAccess = time.Now()
		var cached *botcore.StreamChunk
		// 仅在已完成时返回缓存片段，避免返回半成品。
		if stream.Finished && stream.LastChunk != nil {
			// 拷贝一份，避免外部修改影响缓存。
			clone := *stream.LastChunk
			cached = &clone
		}
		stream.mu.Unlock()
		return snapshot, cached
	}
}

// markFinished 标记会话完成。
// Parameters:
//   - streamID: 会话标识
func (m *StreamManager) markFinished(streamID string) {
	stream := m.getStream(streamID)
	if stream == nil {
		return
	}

	// 标记会话完成以触发清理逻辑。
	stream.setFinished()
}

// setFirstSnapshot 绑定首包快照到会话。
func (m *StreamManager) setFirstSnapshot(streamID string, firstSnapshot botcore.RequestSnapshot) {
	stream := m.getStream(streamID)
	if stream == nil {
		return
	}
	stream.mu.Lock()
	stream.RequestSnapshot = firstSnapshot
	stream.mu.Unlock()
}

// getFirstSnapshot 返回指定会话的首包快照副本。
func (m *StreamManager) getFirstSnapshot(streamID string) botcore.RequestSnapshot {
	stream := m.getStream(streamID)
	if stream == nil {
		return botcore.RequestSnapshot{}
	}
	stream.mu.Lock()
	defer stream.mu.Unlock()
	return stream.RequestSnapshot
}

// getStreamIDByMsg 根据 msgid 获取 streamID，用于消息与会话绑定。
// Parameters:
//   - msgID: 企业微信消息 ID
//
// Returns:
//   - string: 匹配的 streamID
//   - bool: 是否存在对应会话
func (m *StreamManager) getStreamIDByMsg(msgID string) (string, bool) {
	if msgID == "" {
		return "", false
	}

	// 读锁保护下查询消息索引。
	m.mu.RLock()
	streamID, ok := m.msgIndex[msgID]
	m.mu.RUnlock()

	return streamID, ok
}

// cleanup 清理过期的会话。
// 流程图：
//
//	[遍历streams]
//	     |
//	[检查LastAccess超时?] --否--> [跳过]
//	     |
//	    是
//	     |
//	[删除stream及msgIndex映射]
func (m *StreamManager) cleanup() {
	now := time.Now()
	m.mu.Lock()
	// 遍历所有会话，及时清理超时资源。
	for streamID, stream := range m.streams {
		// 会话级别加锁以判断是否已经过期。
		stream.mu.Lock()
		expired := now.Sub(stream.LastAccess) > m.ttl
		stream.mu.Unlock()
		if !expired {
			continue
		}

		// 删除会话以及对应的消息索引。
		delete(m.streams, streamID)
		if stream.MsgID != "" {
			if mapped, ok := m.msgIndex[stream.MsgID]; ok && mapped == streamID {
				delete(m.msgIndex, stream.MsgID)
			}
		}
	}
	m.mu.Unlock()
}

// getStream 根据 streamID 获取会话指针。
// Parameters:
//   - streamID: 会话标识
//
// Returns:
//   - *Stream: 匹配的会话指针，找不到返回 nil
func (m *StreamManager) getStream(streamID string) *Stream {
	if streamID == "" {
		return nil
	}

	// 通过读锁安全获取会话指针。
	m.mu.RLock()
	stream := m.streams[streamID]
	m.mu.RUnlock()

	return stream
}

// touch 更新会话的最后访问时间。
func (s *Stream) touch() {
	// 互斥方式更新最后访问时间，保持会话活跃状态。
	s.mu.Lock()
	s.LastAccess = time.Now()
	s.mu.Unlock()
}

// setFinished 将会话标记为已完成并更新时间。
func (s *Stream) setFinished() {
	// 标记完成并同步刷新最后访问时间，方便后续清理。
	s.mu.Lock()
	s.Finished = true
	s.LastAccess = time.Now()
	s.mu.Unlock()
}

// generateStreamID 生成随机 streamID，失败时回退为时间戳。
func generateStreamID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// 随机源不可用时退化为时间戳，保证唯一性但降低不可预测性。
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}

	// 正常情况下使用 16 字节随机数生成十六进制 streamID。
	return hex.EncodeToString(b)
}
