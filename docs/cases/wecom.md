# 案例：企业微信机器人接入（WeCom）

更新时间：2025-12-14

本页仅把 `pkg/platform/wecom` 作为一个“平台接入案例”说明如何把企业微信回调接到 `IMBotCore` 的 Command 系统上。

## 官方资料（仅引用，不维护）

企业微信机器人接入的官方说明文档（存量资料）放在：

- `docs/appendix/wecom-official/index.md`

当本文与官方文档不一致时，以官方为准。

## 最小接入流程（概念版）

```
WeCom HTTP 回调
  -> wecom.Crypt (验签/解密)
  -> wecom.Bot (http.Handler)
  -> botcore.Chain (路由)
      -> command.Manager (执行指令)
  -> wecom.Emitter (加密并回复)
```

## 最小示例（仅展示关键装配点）

```go
package main

import (
	"log"
	"net/http"
	"time"

	"github.com/IMBotPlatform/IMBotCore/pkg/command"
	"github.com/IMBotPlatform/IMBotCore/pkg/platform/wecom"
	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{Use: "imbot"}
	root.AddCommand(&cobra.Command{
		Use: "ping",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("pong")
			return nil
		},
	})
	return root
}

func main() {
	crypt, err := wecom.NewCrypt("WECOM_TOKEN", "WECOM_ENCODING_AES_KEY", "WECOM_CORP_ID")
	if err != nil {
		log.Fatal(err)
	}

	// command.Manager 作为 pipeline（command 优先场景）。
	manager := command.NewManager(newRootCmd, command.NewMemoryStore())

	bot, err := wecom.NewBot(crypt, time.Minute, 2*time.Second, manager)
	if err != nil {
		log.Fatal(err)
	}

	http.Handle("/callback/command", bot)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
```

> 说明：示例中的 token/aesKey/corpID 需要替换为真实配置；企业微信回调 URL、加解密规则等细节请参考附录官方资料。

