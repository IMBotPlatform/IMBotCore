module github.com/IMBotPlatform/IMBotCore/examples/wecom-claude-code-example

go 1.24.4

require (
	github.com/IMBotPlatform/IMBotCore v0.0.0
	github.com/IMBotPlatform/LLMClaudeCode v1.0.1
	github.com/spf13/cobra v1.10.2
	github.com/tmc/langchaingo v0.1.14
)

require (
	github.com/dlclark/regexp2 v1.10.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/pkoukk/tiktoken-go v0.1.6 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
)

replace github.com/IMBotPlatform/IMBotCore => ../..
