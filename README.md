# IMBotCore

`IMBotCore` 是 `IMBotPlatform` 组织下的核心可复用包集合，提供：

- `pkg/botcore`：统一的 Update / Pipeline / StreamChunk 抽象
- `pkg/command`：基于 Cobra 的命令路由与执行（支持流式输出）
- `pkg/ai`：LLM 能力封装（基于 `langchaingo`）
- `pkg/platform/wecom`：企业微信回调处理与流式响应示例实现

## 安装

```bash
go get github.com/IMBotPlatform/IMBotCore@latest
```

## 添加一个简单 Command（以 `/ping` 为例）

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
		ChatID:    "chat-1",
		SenderID:  "user-1",
		Text:      "/ping",
		Metadata:  map[string]string{"platform": "demo"},
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
- API Reference（自动生成）：`docs/reference/index.md`

本仓库提供 `scripts/generate-docs.sh` 用于本地生成文档；CI 会在 `main` 分支代码变更时自动生成并提交到仓库。

