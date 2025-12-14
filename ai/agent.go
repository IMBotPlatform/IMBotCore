package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tmc/langchaingo/llms"
)

// ToolDefinition 定义工具的接口
type ToolDefinition struct {
	Name        string
	Description string
	Parameters  json.RawMessage // JSON Schema
	Function    func(ctx context.Context, args string) (string, error)
}

// AgentOptions 定义 Agent 运行时的选项
type AgentOptions struct {
	Model      string
	Tools      []ToolDefinition
	MaxTurns   int
	StreamFunc func(string) // 用于实时流式输出中间思考过程或工具结果
}

// RunAgent 运行一个支持工具调用的 Agent 循环
func (s *Service) RunAgent(ctx context.Context, sessionID, prompt string, opts AgentOptions) (string, error) {
	if opts.MaxTurns == 0 {
		opts.MaxTurns = 10
	}

	modelName := opts.Model
	if modelName == "" {
		modelName = s.config.DefaultModel
	}

	llm, err := s.getModel(ctx, modelName)
	if err != nil {
		return "", err
	}

	// 1. 构建初始消息历史
	// 这里我们暂时不复用 Chat() 的存储，因为 Agent 的中间步骤（工具调用）可能非常多，
	// 且不一定适合完全作为聊天历史展示给用户。
	// 但为了上下文，我们应该从 store 中读取最近的历史。
	history, err := s.store.GetHistory(ctx, sessionID)
	if err != nil {
		return "", fmt.Errorf("failed to get history: %w", err)
	}

	var messages []llms.MessageContent
	for _, msg := range history {
		messages = append(messages, llms.TextParts(msg.GetType(), msg.GetContent()))
	}
	// 添加当前用户 prompt
	messages = append(messages, llms.TextParts(llms.ChatMessageTypeHuman, prompt))

	// 2. 转换工具定义
	var llmTools []llms.Tool
	toolMap := make(map[string]ToolDefinition)

	for _, t := range opts.Tools {
		toolMap[t.Name] = t
		llmTools = append(llmTools, llms.Tool{
			Type: "function",
			Function: &llms.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}

	// 3. Agent Loop
	for i := 0; i < opts.MaxTurns; i++ {
		// 调用 LLM
		resp, err := llm.GenerateContent(ctx, messages, llms.WithTools(llmTools))
		if err != nil {
			return "", fmt.Errorf("llm generate error: %w", err)
		}

		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("empty response from llm")
		}

		choice := resp.Choices[0]

		// 检查是否有工具调用
		if len(choice.ToolCalls) > 0 {
			// 将 LLM 的回复（包含工具调用意图）加入历史
			// 注意：LangChainGo 的 MessageContent 处理可能需要手动构建
			// 这里假设 choice.Content 是空的或包含思考过程

			// Log thought process if any
			if choice.Content != "" && opts.StreamFunc != nil {
				opts.StreamFunc(choice.Content + "\n")
			}

			// 添加 Assistant 消息 (包含 ToolCalls)
			msg := llms.MessageContent{
				Role: llms.ChatMessageTypeAI,
				Parts: []llms.ContentPart{
					llms.TextPart(choice.Content),
				},
			}
			for _, tc := range choice.ToolCalls {
				msg.Parts = append(msg.Parts, llms.ToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					FunctionCall: &llms.FunctionCall{
						Name:      tc.FunctionCall.Name,
						Arguments: tc.FunctionCall.Arguments,
					},
				})
			}
			messages = append(messages, msg)

			// 执行所有工具
			for _, tc := range choice.ToolCalls {
				toolName := tc.FunctionCall.Name
				args := tc.FunctionCall.Arguments

				if opts.StreamFunc != nil {
					opts.StreamFunc(fmt.Sprintf("🛠 Executing tool: %s args: %s\n", toolName, args))
				}

				tool, exists := toolMap[toolName]
				if !exists {
					// Tool not found
					messages = append(messages, llms.MessageContent{
						Role: llms.ChatMessageTypeTool,
						Parts: []llms.ContentPart{
							llms.ToolCallResponse{
								ToolCallID: tc.ID,
								Name:       toolName,
								Content:    fmt.Sprintf("Error: Tool %s not found", toolName),
							},
						},
					})
					continue
				}

				// 执行
				result, err := tool.Function(ctx, args)
				if err != nil {
					result = fmt.Sprintf("Error: %v", err)
				}

				if opts.StreamFunc != nil {
					// 截断过长的输出用于展示
					opts.StreamFunc(fmt.Sprintf("✅ Result: %s\n", result))
				}

				// 添加 Tool 结果消息
				messages = append(messages, llms.MessageContent{
					Role: llms.ChatMessageTypeTool,
					Parts: []llms.ContentPart{
						llms.ToolCallResponse{
							ToolCallID: tc.ID,
							Name:       toolName,
							Content:    result,
						},
					},
				})
			}
		} else {
			// 没有工具调用，说明是最终回复
			if opts.StreamFunc != nil {
				opts.StreamFunc(choice.Content)
			}
			return choice.Content, nil
		}
	}

	return "", fmt.Errorf("max turns reached")
}
