# Command 快速上手：新增一个命令并执行

更新时间：2025-12-14

本页演示如何在你的项目中：

1) 定义一个 Cobra 命令树（包含一个 `/ping` 子命令）
2) 创建 `command.Manager`
3) 通过 `Manager.Trigger` 执行并消费流式输出

## 最小示例

```go
package main

import (
	"fmt"

	"github.com/IMBotPlatform/IMBotCore/pkg/botcore"
	imbotcommand "github.com/IMBotPlatform/IMBotCore/pkg/command"
	"github.com/spf13/cobra"
)

// newRootCmd 创建 Cobra root command，并挂载子命令。
// 约定：外部输入用 "/ping"，Parser 会把第一个 token 解析成 "ping"。
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
	update := botcore.Update{
		ChatID:   "chat-1",
		SenderID: "user-1",
		Text:     "/ping",
	}

	manager := imbotcommand.NewManager(newRootCmd, imbotcommand.NewMemoryStore())
	for chunk := range manager.Trigger(update, "stream-1") {
		fmt.Print(chunk.Content)
	}
}
```

## 你应该看到什么

- 控制台打印：`pong`
- `StreamChunk.IsFinal=true` 的结束包会在命令执行结束后自动补发（用于上层平台收尾）

## 下一步

- 理解输出语义与上下文：`docs/guides/command-advanced.md`
- 理解路由组合：`docs/concepts/pipeline.md`

