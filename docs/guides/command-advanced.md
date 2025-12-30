# Command 进阶：上下文、三种回复模式、Responder

更新时间：2025-12-14

本页说明 `pkg/command` 的几个关键能力：

- 如何获取 `ExecutionContext`
- 如何使用上下文存储（ConversationStore）
- 三种被动/主动回复模式

## 1) 获取 ExecutionContext

在 Cobra 的 handler 中：

```go
ctx := command.FromContext(cmd.Context())
```

你可以通过 `ctx.Update` 读取 ChatID/SenderID/Text/Metadata 等信息。

## 2) 上下文存储（ConversationStore）

`Manager` 会在执行前根据 `ConversationKey()` 尝试加载上下文字典到 `ctx.Values`。

如果你要写回：

```go
ctx := command.FromContext(cmd.Context())
key := ctx.ConversationKey()

if ctx.Store != nil {
	_ = ctx.Store.Save(key, command.ContextValues{
		"last_command": "demo",
	})
}
```

## 3) 三种回复模式

### 模式 A：文本流（默认）

只要 `cmd.Print*` 输出，就会自动转成 `StreamChunk.Content` 流式片段：

```go
cmd.Println("第一段输出")
cmd.Println("第二段输出")
```

### 模式 B：Payload（同步回复复杂对象）

当你希望上层平台收到“一个结构化对象”（而不是文本流）：

```go
ctx := command.FromContext(cmd.Context())
ctx.SetResponsePayload(map[string]any{"ok": true})
```

上层平台需要根据 `StreamChunk.Payload` 决定如何序列化/加密/返回（例如企业微信的卡片消息）。

### 模式 C：NoResponse（静默执行 + 主动推送）

当你不希望产生任何被动回复（例如避免对话框出现气泡），改用主动推送：

```go
ctx := command.FromContext(cmd.Context())
ctx.SetNoResponse()

if ctx.Responder() != nil {
	responseURL := ctx.Update.Metadata["response_url"] // 以企业微信为例
	_ = ctx.Responder().SendMarkdown(responseURL, "# 异步通知\n任务已后台开始")
}
```

## 下一步

- 企业微信接入案例（并附官方资料索引）：`docs/cases/wecom.md`
