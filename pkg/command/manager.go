package command

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/IMBotPlatform/IMBotCore/pkg/botcore"
)

// Manager 实现 PipelineInvoker，负责串联解析、构建 Cobra 命令树并执行。
type Manager struct {
	factory CommandFunc
	parser  Parser
	logger  *log.Logger

	responser botcore.Responser
}

// ManagerOption 自定义 Manager 行为。
type ManagerOption func(*Manager)

// WithLogger 注入自定义日志记录器。
func WithLogger(l *log.Logger) ManagerOption {
	return func(m *Manager) {
		m.logger = l
	}
}

// WithResponser 注入主动消息发送器（当 PipelineContext.Responser 为空时作为兜底）。
func WithResponser(r botcore.Responser) ManagerOption {
	return func(m *Manager) {
		m.responser = r
	}
}

// NewManager 绑定命令构建函数，返回实现 PipelineInvoker 的管理器。
func NewManager(factory CommandFunc, opts ...ManagerOption) *Manager {
	mgr := &Manager{
		factory: factory,
		parser:  NewParser(), // 保留 Parser 用于判断是否为命令（前缀检查）
	}
	for _, opt := range opts {
		opt(mgr)
	}
	return mgr
}

// Trigger 满足 botcore.PipelineInvoker，为每个请求构建独立的命令树并执行。
func (m *Manager) Trigger(pipelineCtx botcore.PipelineContext) <-chan botcore.StreamChunk {
	outCh := make(chan botcore.StreamChunk, 1)
	go func() {
		defer close(outCh)

		if m == nil || m.factory == nil {
			outCh <- botcore.StreamChunk{Content: "Error: Command Manager not initialized", IsFinal: true}
			return
		}

		update := pipelineCtx.Snapshot
		// 1. 初步解析
		parsed := m.parser.Parse(update.Text)
		if !parsed.IsCommand {
			if strings.TrimSpace(update.Text) == "" {
				outCh <- botcore.StreamChunk{Content: "请输入命令 (e.g. /help)", IsFinal: true}
			} else {
				outCh <- botcore.StreamChunk{Content: fmt.Sprintf("未识别的命令: %s\n请尝试 /help", parsed.Raw), IsFinal: true}
			}
			return
		}

		// 2. 创建 Cobra 命令树
		rootCmd := m.factory()

		// 3. 配置 IO 重定向
		writer := NewStreamWriter(outCh)
		rootCmd.SetOut(writer)
		rootCmd.SetErr(writer)
		rootCmd.CompletionOptions.DisableDefaultCmd = true

		// 4. 准备上下文
		execCtx := &ExecutionContext{
			RequestSnapshot: update,
			ch:              outCh,
			responser:       pipelineCtx.Responser,
		}
		if execCtx.responser == nil {
			execCtx.responser = m.responser
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
			outCh <- botcore.StreamChunk{Content: fmt.Sprintf("❌ 执行出错: %v\n", err)}
		}

		// 执行结束后，如果没有发送过任何显式信号，也没有流式输出（StreamWriter自动处理），
		// 这里发送一个默认的结束包。
		// 注意：StreamWriter 在 Write 时会发送非 Final 包。
		// 这里我们只负责最后的兜底。
		execCtx.sendFinal(botcore.StreamChunk{Content: "", IsFinal: true})
	}()
	return outCh
}

func (m *Manager) logf(format string, args ...any) {
	if m == nil || m.logger == nil {
		return
	}
	m.logger.Printf(format, args...)
}
