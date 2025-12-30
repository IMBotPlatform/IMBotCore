package command

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/IMBotPlatform/IMBotCore/pkg/botcore"
)

const commandLogSnippet = 256

// Manager 实现 PipelineInvoker，负责串联解析、构建 Cobra 命令树并执行。
type Manager struct {
	factory   CommandFactory
	parser    Parser
	store     ConversationStore
	logger    *log.Logger
	responder botcore.ActiveResponder
}

// ManagerOption 自定义 Manager 行为。
type ManagerOption func(*Manager)

// WithLogger 注入自定义日志记录器。
func WithLogger(l *log.Logger) ManagerOption {
	return func(m *Manager) {
		m.logger = l
	}
}

// WithResponder 注入主动消息发送器。
func WithResponder(r botcore.ActiveResponder) ManagerOption {
	return func(m *Manager) {
		m.responder = r
	}
}

// NewManager 绑定命令工厂与存储，返回实现 PipelineInvoker 的管理器。
func NewManager(factory CommandFactory, store ConversationStore, opts ...ManagerOption) *Manager {
	mgr := &Manager{
		factory: factory,
		parser:  NewParser(), // 保留 Parser 用于判断是否为命令（前缀检查）
		store:   store,
	}
	for _, opt := range opts {
		opt(mgr)
	}
	return mgr
}

// Trigger 满足 botcore.PipelineInvoker，为每个请求构建独立的命令树并执行。
func (m *Manager) Trigger(update botcore.Update, streamID string) <-chan botcore.StreamChunk {
	out := make(chan botcore.StreamChunk, 1)
	go func() {
		defer close(out)

		if m == nil || m.factory == nil {
			out <- botcore.StreamChunk{Content: "Error: Command Manager not initialized", IsFinal: true}
			return
		}

		// 1. 初步解析
		parsed := m.parser.Parse(update.Text)
		if !parsed.IsCommand {
			if strings.TrimSpace(update.Text) == "" {
				out <- botcore.StreamChunk{Content: "请输入命令 (e.g. /help)", IsFinal: true}
			} else {
				out <- botcore.StreamChunk{Content: fmt.Sprintf("未识别的命令: %s\n请尝试 /help", parsed.Raw), IsFinal: true}
			}
			return
		}

		// 2. 创建 Cobra 命令树
		rootCmd := m.factory()

		// 3. 配置 IO 重定向
		writer := NewStreamWriter(out)
		rootCmd.SetOut(writer)
		rootCmd.SetErr(writer)
		rootCmd.CompletionOptions.DisableDefaultCmd = true

		// 4. 准备上下文
		// 使用 sync.Once 确保 Final 信号只发送一次（无论是通过 Explicit Signal 还是通过 Writer 关闭）
		var signalOnce sync.Once
		sendSignal := func(chunk botcore.StreamChunk) {
			signalOnce.Do(func() {
				// 如果是显式信号，直接发送
				out <- chunk
			})
		}

		execCtx := &ExecutionContext{
			Update:     update,
			StreamID:   streamID,
			Store:      m.store,
			responder:  m.responder,
			sendSignal: sendSignal,
		}

		convKey := execCtx.ConversationKey()
		if m.store != nil {
			if values, err := m.store.Load(convKey); err != nil {
				m.logf("上下文加载失败: %v", err)
			} else {
				execCtx.Values = values
			}
		}

		ctx := WithExecutionContext(context.Background(), execCtx)

		// 5. 设置参数并执行
		args := parsed.Tokens
		// 如果第一个 token 匹配 root command 的 name，移除它以避免 "unknown command X for X" 错误
		if len(args) > 0 && strings.EqualFold(args[0], rootCmd.Name()) {
			args = args[1:]
		}
		rootCmd.SetArgs(args)
		m.logf("Executing command: %v for user %s", args, update.SenderID)

		if err := rootCmd.ExecuteContext(ctx); err != nil {
			m.logf("Command execution error: %v", err)
			out <- botcore.StreamChunk{Content: fmt.Sprintf("❌ 执行出错: %v\n", err)}
		}

		// 执行结束后，如果没有发送过任何显式信号，也没有流式输出（StreamWriter自动处理），
		// 这里发送一个默认的结束包。
		// 注意：StreamWriter 在 Write 时会发送非 Final 包。
		// 这里我们只负责最后的兜底。
		signalOnce.Do(func() {
			out <- botcore.StreamChunk{Content: "", IsFinal: true}
		})
	}()
	return out
}

func (m *Manager) logf(format string, args ...interface{}) {
	if m == nil || m.logger == nil {
		return
	}
	m.logger.Printf(format, args...)
}

// truncateForLog 限制日志中输出的文本长度。
func truncateForLog(src string, limit int) string {
	if limit <= 0 || len(src) <= limit {
		return src
	}
	return fmt.Sprintf("%s...(truncated)", src[:limit])
}
