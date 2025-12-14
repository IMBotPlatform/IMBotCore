package ai

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/tmc/langchaingo/llms/openai"
)

// Service 是 AI 逻辑的主要入口点。
// 它负责管理模型实例、会话状态以及与 LLM 的交互。
type Service struct {
	config     *Config
	store      SessionStore
	modelCache map[string]llms.Model
}

// NewService 创建一个新的 AI 服务实例。
func NewService(config *Config, store SessionStore) *Service {
	return &Service{
		config:     config,
		store:      store,
		modelCache: make(map[string]llms.Model),
	}
}

// resolveAPIKey 解析 API 密钥。
// 如果密钥以 "env:" 开头，则从环境变量中获取实际值。
func resolveAPIKey(key string) string {
	if strings.HasPrefix(key, "env:") {
		return os.Getenv(strings.TrimPrefix(key, "env:"))
	}
	return key
}

// getModel 获取模型实例。
// 如果缓存中存在则直接返回，否则初始化一个新的模型实例并缓存。
//
// 逻辑流程:
// Check Cache -> (Hit) -> Return
//
//	  |
//	(Miss)
//	  v
//
// Load Config -> Init Provider (OpenAI/Google) -> Update Cache -> Return
func (s *Service) getModel(ctx context.Context, modelName string) (llms.Model, error) {
	if model, ok := s.modelCache[modelName]; ok {
		return model, nil
	}

	var cfg *ModelConfig
	for i := range s.config.Models {
		if s.config.Models[i].Name == modelName {
			cfg = &s.config.Models[i]
			break
		}
	}
	if cfg == nil {
		return nil, fmt.Errorf("model '%s' not found in configuration", modelName)
	}

	var llm llms.Model
	var err error

	apiKey := resolveAPIKey(cfg.APIKey)

	switch cfg.Provider {
	case "openai":
		llm, err = openai.New(
			openai.WithToken(apiKey),
			openai.WithModel(cfg.ModelName),
			openai.WithBaseURL(cfg.BaseURL),
		)
	case "google":
		llm, err = googleai.New(ctx,
			googleai.WithAPIKey(apiKey),
			googleai.WithDefaultModel(cfg.ModelName),
		)
	case "anthropic":
		opts := []anthropic.Option{
			anthropic.WithToken(apiKey),
			anthropic.WithModel(cfg.ModelName),
		}
		if cfg.BaseURL != "" {
			opts = append(opts, anthropic.WithBaseURL(cfg.BaseURL))
		}
		llm, err = anthropic.New(opts...)
	// TODO: 后续添加 ollama 支持
	default:
		return nil, fmt.Errorf("unsupported provider: %s", cfg.Provider)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create model provider: %w", err)
	}

	s.modelCache[modelName] = llm
	return llm, nil
}

// ChatOptions 定义调用 Chat 时的配置。
type ChatOptions struct {
	Model string
}

// ChatOption 是配置 ChatOptions 的函数。
type ChatOption func(*ChatOptions)

// WithModel 指定使用的模型。
func WithModel(model string) ChatOption {
	return func(o *ChatOptions) {
		o.Model = model
	}
}

// Chat 处理用户的消息，与 LLM 交互，并返回流式响应。
//
// 核心架构流程图:
//
//	User Input (String)
//	      |
//	      v
//	+-------------------------+
//	| SessionStore (Memory)   |
//	| 1. Save User Message    |
//	| 2. Load Full History    |
//	+-----------+-------------+
//	            |
//	            v
//	+-------------------------+
//	| LLM Provider (OpenAI/..) |
//	| 3. StreamChat()         |
//	+-----------+-------------+
//	            |
//	            +-------------------------> [Output Channel] -> (Stream to User)
//	            |
//	            v
//	+-------------------------+
//	| SessionStore            |
//	| 4. Save AI Response     |
//	+-------------------------+
func (s *Service) Chat(ctx context.Context, sessionID, prompt string, opts ...ChatOption) (<-chan string, error) {
	// Step 0: 解析选项（默认使用配置中的 default_model，可被 WithModel 覆盖）
	options := &ChatOptions{
		Model: s.config.DefaultModel,
	}
	for _, o := range opts {
		o(options)
	}

	// Step 1: 获取指定的模型（若调用方未设置则回退 default_model）
	modelName := options.Model
	if modelName == "" {
		modelName = s.config.DefaultModel
	}

	llm, err := s.getModel(ctx, modelName)
	if err != nil {
		return nil, err
	}

	// Step 2: 将用户的新消息保存到会话存储中
	if err := s.store.AddUserMessage(ctx, sessionID, prompt); err != nil {
		return nil, fmt.Errorf("failed to add user message: %w", err)
	}

	// Step 3: 加载完整的历史对话记录
	// TODO: 长对话可在此处增加窗口裁剪以控制 Token 大小。
	history, err := s.store.GetHistory(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}

	// 转为 GenerateContent 所需的消息片段
	var contentMessages []llms.MessageContent
	for _, msg := range history {
		contentMessages = append(contentMessages, llms.TextParts(msg.GetType(), msg.GetContent()))
	}

	stream := make(chan string)

	// Step 4: 异步调用 LLM，流式写回 token
	go func() {
		defer close(stream)

		var fullResponse strings.Builder

		_, err := llm.GenerateContent(
			ctx,
			contentMessages,
			llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
				content := string(chunk)
				stream <- content         // 流式返回
				fullResponse.Write(chunk) // 累计全量用于落库
				return nil
			}),
		)

		if err != nil {
			// 轻量级错误透传；生产可结合日志/指标
			fmt.Printf("Streaming error: %v\n", err)
			stream <- fmt.Sprintf("\n[AI_ERROR]: %v", err)
			return
		}

		// Step 5: 将完整回复保存到会话存储中（异步阶段）
		if fullResponse.Len() > 0 {
			if err := s.store.AddAIMessage(context.Background(), sessionID, fullResponse.String()); err != nil {
				// 已经返回给用户，存储失败只记录
				fmt.Printf("Failed to save AI message to store: %v\n", err)
			}
		}
	}()

	return stream, nil
}
