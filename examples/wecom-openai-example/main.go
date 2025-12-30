package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/IMBotPlatform/IMBotCore/pkg/botcore"
	"github.com/IMBotPlatform/IMBotCore/pkg/command"
	"github.com/IMBotPlatform/IMBotCore/pkg/platform/wecom"
	"github.com/spf13/cobra"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

const (
	defaultListenAddr = ":8080"
)

// envConfig 存放示例所需的环境变量配置。
type envConfig struct {
	wecomToken  string
	wecomAESKey string
	wecomCorpID string

	openAIKey     string
	openAIModel   string
	openAIBaseURL string

	listenAddr string
}

// newRootCmd 构建 Cobra 命令树。
// 参数：llm 为 langchaingo 模型实例（用于 /ai 命令）。
// 返回：*cobra.Command 根命令。
func newRootCmd(llm llms.Model) *cobra.Command {
	root := &cobra.Command{
		Use:           "imbot",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(&cobra.Command{
		Use:   "ping",
		Short: "健康检查",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("pong")
			return nil
		},
	})

	root.AddCommand(&cobra.Command{
		Use:   "echo <text>",
		Short: "回显输入文本",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println(strings.Join(args, " "))
			return nil
		},
	})

	aiCmd := &cobra.Command{
		Use:   "ai <prompt>",
		Short: "调用 LLM 并流式输出",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if llm == nil {
				return fmt.Errorf("llm not initialized")
			}

			prompt := strings.Join(args, " ")
			ch, err := streamPrompt(cmd.Context(), llm, prompt)
			if err != nil {
				return err
			}
			for part := range ch {
				cmd.Print(part)
			}
			return nil
		},
	}
	root.AddCommand(aiCmd)

	return root
}

// newAIHandler 构建默认 AI 路由（非命令消息）。
// 参数：llm 为 langchaingo 模型实例。
// 返回：botcore.PipelineInvoker。
func newAIHandler(llm llms.Model) botcore.PipelineInvoker {
	return botcore.PipelineFunc(func(update botcore.Update, streamID string) <-chan botcore.StreamChunk {
		out := make(chan botcore.StreamChunk, 1)
		go func() {
			defer close(out)

			prompt := strings.TrimSpace(update.Text)
			if prompt == "" {
				out <- botcore.StreamChunk{Content: "empty input", IsFinal: true}
				return
			}
			if llm == nil {
				out <- botcore.StreamChunk{Content: "llm not initialized", IsFinal: true}
				return
			}

			// 调用 LLM 并将流式输出转换为 StreamChunk。
			ch, err := streamPrompt(context.Background(), llm, prompt)
			if err != nil {
				out <- botcore.StreamChunk{Content: fmt.Sprintf("ai error: %v", err), IsFinal: true}
				return
			}

			for part := range ch {
				out <- botcore.StreamChunk{Content: part}
			}
			out <- botcore.StreamChunk{Content: "", IsFinal: true}
		}()
		return out
	})
}

// loadEnvConfig 统一读取并校验示例所需环境变量。
// 返回：envConfig；缺失必需变量时直接退出。
func loadEnvConfig() envConfig {
	cfg := envConfig{
		wecomToken:    strings.TrimSpace(os.Getenv("WECOM_TOKEN")),
		wecomAESKey:   strings.TrimSpace(os.Getenv("WECOM_ENCODING_AES_KEY")),
		wecomCorpID:   strings.TrimSpace(os.Getenv("WECOM_CORP_ID")),
		openAIKey:     strings.TrimSpace(os.Getenv("OPENAI_API_KEY")),
		openAIModel:   strings.TrimSpace(os.Getenv("OPENAI_MODEL")),
		openAIBaseURL: strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")),
		listenAddr:    strings.TrimSpace(os.Getenv("LISTEN_ADDR")),
	}

	var missing []string
	if cfg.wecomToken == "" {
		missing = append(missing, "WECOM_TOKEN")
	}
	if cfg.wecomAESKey == "" {
		missing = append(missing, "WECOM_ENCODING_AES_KEY")
	}
	if cfg.wecomCorpID == "" {
		missing = append(missing, "WECOM_CORP_ID")
	}
	if cfg.openAIKey == "" {
		missing = append(missing, "OPENAI_API_KEY")
	}
	if len(missing) > 0 {
		log.Fatalf("missing env: %s", strings.Join(missing, ", "))
	}
	if cfg.listenAddr == "" {
		cfg.listenAddr = defaultListenAddr
	}
	return cfg
}

func main() {
	// 1) 读取并校验环境变量。
	cfg := loadEnvConfig()

	// 2) 初始化 LLM（langchaingo）。
	llm, err := newOpenAILLM(cfg.openAIKey, cfg.openAIModel, cfg.openAIBaseURL)
	if err != nil {
		log.Fatalf("init llm: %v", err)
	}

	// 3) 构建命令管理器与默认 AI 路由。
	manager := command.NewManager(
		func() *cobra.Command {
			return newRootCmd(llm)
		},
		command.NewMemoryStore(),
		command.WithResponder(wecom.NewClient()),
	)
	aiHandler := newAIHandler(llm)

	// 4) 构建路由链。
	chain := botcore.NewChain(aiHandler)
	chain.AddRoute("command", botcore.MatchPrefix("/"), manager)

	// 5) 初始化企业微信 Bot。
	crypt, err := wecom.NewCrypt(cfg.wecomToken, cfg.wecomAESKey, cfg.wecomCorpID)
	if err != nil {
		log.Fatalf("init wecom crypt: %v", err)
	}
	bot, err := wecom.NewBot(crypt, time.Minute, 2*time.Second, chain)
	if err != nil {
		log.Fatalf("init wecom bot: %v", err)
	}

	// 6) 启动 HTTP 服务。
	http.Handle("/callback/command", bot)
	log.Printf("wecom example listening on %s", cfg.listenAddr)
	log.Fatal(http.ListenAndServe(cfg.listenAddr, nil))
}

// streamPrompt 以流式方式调用 LLM。
// 参数：ctx 为上下文，llm 为模型实例，prompt 为输入文本。
// 返回：输出片段通道与错误。
func streamPrompt(ctx context.Context, llm llms.Model, prompt string) (<-chan string, error) {
	if llm == nil {
		return nil, fmt.Errorf("llm is nil")
	}
	if strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("prompt is empty")
	}

	out := make(chan string)
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	}
	go func() {
		defer close(out)
		_, err := llm.GenerateContent(ctx, messages, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			out <- string(chunk)
			return nil
		}))
		if err != nil {
			out <- fmt.Sprintf("\n[LLM_ERROR]: %v", err)
		}
	}()
	return out, nil
}

// newOpenAILLM 创建 OpenAI 模型实例。
// 参数：apiKey 为必需，model/baseURL 为可选。
// 返回：llms.Model 与错误。
func newOpenAILLM(apiKey, model, baseURL string) (llms.Model, error) {
	opts := []openai.Option{
		openai.WithToken(apiKey),
	}
	if model != "" {
		opts = append(opts, openai.WithModel(model))
	}
	if baseURL != "" {
		opts = append(opts, openai.WithBaseURL(baseURL))
	}
	return openai.New(opts...)
}
