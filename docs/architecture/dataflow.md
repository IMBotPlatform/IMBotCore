# 回调与流式数据流

更新时间：2026-01-30

## 时序概览（文字版）
1) 企业微信推送加密消息到 `/callback/command`。
2) `wecom.Bot` 验签 + 解密，得到标准化 `RequestSnapshot`（含 ChatType/ChatID/Text/Attachments/Metadata，ID=streamID）。
3) 进入 `botcore.Chain` 路由：前缀为 `/` -> 命令；否则 -> AI。
4) 处理结果通过 `StreamManager` 组装流式片段，由 `wecom.Bot.BuildReply` 编码明文，再由 `wecom.Crypt` 加密后回写企业微信；首包为空 ACK，刷新请求拉取最新片段。

## 流程 ASCII
```
Enterprise WeCom (POST encrypted)
          |
          v
      Gin /callback/command
          |
          v
       wecom.Bot
    [verify + decrypt + normalize]
          |
          v
  wecom.StreamManager
    - create streamID -> RequestSnapshot.ID
    - publish / drain latest chunk
          |
    +-----+--------------------+
    | Prefix "/" ?             |
    |                          |
Yes v                          v No
CommandManager            AIHandler
  -> Cobra cmds             -> 应用侧 LLM（流式）
  -> ctx.Send*              ->（应用侧可选存储）
          \                    /
           \                  /
            \                /
             v              v
             wecom.Bot.BuildReply
             - wrap chunks
             - encrypt (wecom.Crypt)
             v
      HTTP Response / refresh
```

## 加解密与安全要点
- 加解密：`wecom.NewBot` 内部调用 `wecom.NewCrypt`，使用 `WECOM_TOKEN`、`WECOM_ENCODING_AES_KEY`、`WECOM_CORP_ID`。
- 企业微信要求 5s 内响应：首包返回空 ACK 避免阻塞；刷新等待由 `WECOM_BOT_STREAM_WAIT_TIMEOUT` 控制，流会话存活由 `WECOM_BOT_STREAM_TTL` 控制。
- 主动回复请求超时由 `WECOM_BOT_HTTP_TIMEOUT` 控制。
- 历史落盘：IMBotCore 不内置历史存储，是否落盘由应用侧自行决定。

## 指令与回调路径说明
- 回调 URL 由应用侧绑定（示例为 `/callback/command`），企业微信后台需配置一致。
- 按钮/卡片回调会把 `EventKey` 映射为命令文本（如 `/demo callback approve`），进入同一路由逻辑。

## 相关代码定位
- 平台接入（企业微信案例）：`pkg/platform/wecom/bot.go`, `pkg/platform/wecom/stream.go`, `pkg/platform/wecom/message.go`
- 路由/链：`pkg/botcore/chain.go`
- 命令系统：`pkg/command/manager.go`, `pkg/command/parser.go`, `pkg/command/context.go`
- AI 流式：应用侧使用 langchaingo 或其他 SDK 实现
