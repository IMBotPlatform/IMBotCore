# IMBotCore

`IMBotCore` 是 `IMBotPlatform` 组织下的即时通讯机器人核心库：提供平台无关的 **Update / Pipeline / StreamChunk** 抽象与基于 Cobra 的 **Command 指令系统**，支持**流式输出**与上下文能力。

## 特性

- **统一输入**：`pkg/botcore` 提供 `Update`（平台适配器输出）与 `Chain`（责任链路由）。
- **指令系统**：`pkg/command` 提供解析、命令树工厂、执行上下文与三种回复语义（文本流 / Payload / NoResponse）。
- **流式输出**：以 `<-chan StreamChunk` 的方式向上游平台持续产出片段，并以 `IsFinal=true` 明确结束。
- **平台案例**：`pkg/platform/wecom` 提供企业微信回调处理与流式响应参考实现（案例，不绑定你的框架/部署方式）。

## 快速开始

### 安装

```bash
go get github.com/IMBotPlatform/IMBotCore@latest
```

### 最小示例：添加一个 `/ping` 命令

下面演示如何创建一个最小命令树，并用 `command.Manager` 执行。

```go
package main

import (
	"fmt"

	"github.com/IMBotPlatform/IMBotCore/pkg/botcore"
	imbotcommand "github.com/IMBotPlatform/IMBotCore/pkg/command"
	"github.com/spf13/cobra"
)

// newRootCmd 创建 Cobra root command，并挂载子命令。
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "imbot",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(&cobra.Command{
		Use:   "ping",
		Short: "health check",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("pong")
			return nil
		},
	})

	return root
}

func main() {
	// 注意：Update.Text 以 "/" 前缀表示命令，例如 "/ping"。
	update := botcore.Update{
		ChatID:   "chat-1",
		SenderID: "user-1",
		Text:     "/ping",
		Metadata: map[string]string{"platform": "demo"},
	}

	manager := imbotcommand.NewManager(newRootCmd, imbotcommand.NewMemoryStore())
	for chunk := range manager.Trigger(update, "stream-1") {
		// chunk.Content 为流式输出内容；chunk.IsFinal 为结束信号。
		fmt.Print(chunk.Content)
	}
}
```

## 文档

- 手写文档入口：`docs/index.md`
- 架构总览：`docs/architecture/overview.md`
- API Reference（自动生成）：`docs/reference/index.md`

本仓库提供 `scripts/generate-docs.sh` 用于本地生成 `docs/reference`；CI 会在 `main` 分支代码变更时自动生成并提交到仓库。

## 开发

```bash
go test ./...
scripts/generate-docs.sh          # 生成 API Reference
scripts/generate-docs.sh --check  # 校验 docs/reference 是否最新
```

## 贡献

请先阅读：`CONTRIBUTING.md`

## 安全

安全问题请参考：`SECURITY.md`

## License

本项目采用 `GNU AGPL-3.0` 开源许可证，详见：`LICENSE`
