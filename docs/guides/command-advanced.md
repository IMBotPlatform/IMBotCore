# Command 进阶：ExecutionContext、三种回复模式、主动推送

更新时间：2026-01-30

本页说明 `pkg/command` 的几个关键能力：

- 如何获取 `ExecutionContext`
- 三种被动/主动回复模式

## 1) 获取 ExecutionContext

在 Cobra 的 handler 中：

```go
ctx := command.FromContext(cmd.Context())
```

你可以通过 `ctx.RequestSnapshot` 读取 ChatID/SenderID/Text/Metadata 等信息。

## 2) 三种回复模式

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
ctx.SendPayload(map[string]any{"ok": true})
```

上层平台需要根据 `StreamChunk.Payload` 决定如何序列化/加密/返回（例如企业微信的卡片消息）。

### 模式 C：NoResponse（静默执行 + 主动推送）

当你不希望产生任何被动回复（例如避免对话框出现气泡），改用主动推送：

```go
ctx := command.FromContext(cmd.Context())
ctx.SendNoResponse()

// ResponseMarkdown 会自动使用 RequestSnapshot.ResponseURL。
_ = ctx.ResponseMarkdown("# 异步通知\n任务已后台开始")
```

## 下一步

- 企业微信接入案例（并附官方资料索引）：`docs/cases/wecom.md`
