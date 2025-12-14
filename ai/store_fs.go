package ai

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/tmc/langchaingo/llms"
)

// storedMessage 是用于 JSON 序列化的中间结构
type storedMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// FileStore 实现了基于文件系统的 SessionStore (JSONL 格式)。
// 每个 Session 的历史记录存储在单独的文件中，每行一个 JSON 对象。
type FileStore struct {
	baseDir string
	mu      sync.RWMutex // 全局锁，保护文件系统操作并发安全
}

// NewFileStore 创建一个新的 FileStore。
// baseDir: 存储历史记录的目录路径。
func NewFileStore(baseDir string) (*FileStore, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create history directory: %w", err)
	}
	return &FileStore{
		baseDir: baseDir,
	}, nil
}

// getFilePath 返回指定 SessionID 的文件路径。
// 对 SessionID 进行简单的清理以防路径遍历。
// 后缀改为 .jsonl 以示区别
func (s *FileStore) getFilePath(sessionID string) string {
	safeID := filepath.Base(sessionID)
	return filepath.Join(s.baseDir, safeID+".jsonl")
}

// appendToFile 追加一行 JSON 记录到文件
func (s *FileStore) appendToFile(path string, msg llms.ChatMessage) error {
	// 转换为中间结构
	role := "system"
	switch msg.GetType() {
	case llms.ChatMessageTypeHuman:
		role = "user"
	case llms.ChatMessageTypeAI:
		role = "ai"
	case llms.ChatMessageTypeSystem:
		role = "system"
	}

	sm := storedMessage{
		Role:    role,
		Content: msg.GetContent(),
	}

	// 以追加模式打开文件，如果不存在则创建
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// 写入 JSON Line
	// json.Encoder 默认会在末尾加 \n，符合 JSONL 规范
	encoder := json.NewEncoder(f)
	encoder.SetEscapeHTML(false) // 保持原始字符，不转义 <, >, &
	return encoder.Encode(sm)
}

// GetHistory 逐行读取文件获取历史记录
func (s *FileStore) GetHistory(ctx context.Context, sessionID string) ([]llms.ChatMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := s.getFilePath(sessionID)
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return []llms.ChatMessage{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var messages []llms.ChatMessage
	scanner := bufio.NewScanner(f)

	// 增加 Buffer 大小以支持超长单行（默认 64KB 可能不够）
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 5*1024*1024) // 设置最大单行限制为 5MB

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var sm storedMessage
		if err := json.Unmarshal(line, &sm); err != nil {
			// 遇到坏行，打印警告并跳过，保证最大容错性
			fmt.Printf("Warning: skipping malformed line %d in %s: %v\n", lineNum, path, err)
			continue
		}

		switch sm.Role {
		case "user":
			messages = append(messages, llms.HumanChatMessage{Content: sm.Content})
		case "ai":
			messages = append(messages, llms.AIChatMessage{Content: sm.Content})
		case "system":
			messages = append(messages, llms.SystemChatMessage{Content: sm.Content})
		default:
			messages = append(messages, llms.SystemChatMessage{Content: fmt.Sprintf("[%s]: %s", sm.Role, sm.Content)})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning history file: %w", err)
	}

	return messages, nil
}

// AddUserMessage 添加用户消息（追加写入）
func (s *FileStore) AddUserMessage(ctx context.Context, sessionID, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.appendToFile(s.getFilePath(sessionID), llms.HumanChatMessage{Content: text})
}

// AddAIMessage 添加 AI 消息（追加写入）
func (s *FileStore) AddAIMessage(ctx context.Context, sessionID, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.appendToFile(s.getFilePath(sessionID), llms.AIChatMessage{Content: text})
}

// ClearHistory 清空会话历史（删除文件）
func (s *FileStore) ClearHistory(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return os.Remove(s.getFilePath(sessionID))
}