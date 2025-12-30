# 回调与流式数据流

更新时间：2025-12-16

## 时序概览（文字版）
1) 企业微信推送加密消息到 `/callback/command`。
2) `wecom.Bot` 验签 + 解密，得到标准化 `Update`（含 ChatType/ChatID/Text/Metadata）。
3) 进入 `botcore.Chain` 路由：前缀为 `/` -> 命令；否则 -> AI。
4) 处理结果通过 `SessionManager` 组装流式片段，调用 `wecom.Client` 加密后回写企业微信；首包兜底确保企业微信侧刷新。

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
  botcore.SessionManager
    - create streamID
    - refresh / timeout guard
          |
    +-----+--------------------+
    | Prefix "/" ?             |
    |                          |
Yes v                          v No
CommandManager            AIHandler
  -> Cobra cmds             -> 应用侧 LLM（流式）
  -> Responder              ->（应用侧可选存储）
          \                    /
           \                  /
            \                /
             v              v
             StreamEmitter
             - wrap chunks
             - encrypt (wecom.Crypt)
             v
      HTTP Response / refresh
```

## 加解密与安全要点
- 加解密：`wecom.NewCrypt` 使用 `WECOM_TOKEN`、`WECOM_ENCODING_AES_KEY`、`WECOM_CORP_ID`。
- 企业微信要求 5s 内响应：流式输出首包由 SessionManager 保障，刷新超时由 `WECOM_REFRESH_TIMEOUT` 控制。
- 历史落盘：IMBotCore 不内置历史存储，是否落盘由应用侧自行决定。

## 指令与回调路径说明
- 回调 URL 固定 `/callback/command`，企业微信后台需与上述 ENV 保持一致。
- 按钮/卡片回调会把 `EventKey` 映射为命令文本（如 `/demo callback approve`），进入同一路由逻辑。

## 相关代码定位
- 平台接入（企业微信案例）：`pkg/platform/wecom/bot.go`, `pkg/platform/wecom/adapter.go`, `pkg/platform/wecom/session.go`
- 路由/链：`pkg/botcore/router.go`
- 命令系统：`pkg/command/manager.go`, `pkg/command/parser.go`, `pkg/command/context.go`
- AI 流式：应用侧使用 langchaingo 或其他 SDK 实现
