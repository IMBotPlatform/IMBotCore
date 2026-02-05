# Pipeline / Router 概念说明

更新时间：2026-01-30

本页解释 `pkg/botcore` 的核心抽象，用于把平台输入统一成 `RequestSnapshot`，再通过 `PipelineContext` 交给不同的处理器（例如 `command.Manager`）。

## 关键抽象

### 1) RequestSnapshot

`botcore.RequestSnapshot` 是平台无关的首包请求快照结构，包含：

- `ID`：平台内的唯一消息 / 事件 / 流会话 ID（如 wecom 的 `streamID`）
- `Text`：文本内容（常用于命令匹配）
- `ChatID` / `SenderID` / `ChatType`：会话与用户维度
- `Attachments`：标准化附件（图片 / 文件），可调用 `SaveAttachments` 落盘（超时由 `WECOM_BOT_SAVE_ATTACH_TIMEOUT` 控制）
- `ResponseURL`：主动回复地址（若平台支持）
- `Raw`：保留平台原始结构（必要时可深入）
- `Metadata`：扩展字段（例如 `platform`、`event_type` 等）

### 2) StreamChunk

`botcore.StreamChunk` 是处理器输出的统一格式：

- `Content`：文本增量（可多次发送）
- `Payload`：复杂对象（例如卡片/结构化响应）
- `IsFinal`：结束信号
- `Payload == botcore.NoResponse`：表示无需被动回复（交由平台实现处理）

### 3) PipelineContext

`botcore.PipelineContext` 是 Pipeline 的显式上下文容器，包含：

- `Snapshot`：标准化首包快照
- `Responser`：主动回复能力（可为空）

### 4) PipelineInvoker

`PipelineInvoker` 是统一的“执行器”接口：

```go
Trigger(ctx PipelineContext) <-chan StreamChunk
```

其中流式会话标识统一由 `ctx.Snapshot.ID` 表达（例如 wecom 的 `streamID`）。

### 5) Chain（责任链路由）

`botcore.Chain` 提供一个最小路由器：

- 按顺序检查每条 `Route.Matcher`
- 命中则交给对应 `PipelineInvoker`
- 否则交给 `defaultHandler`

最常见的策略：

- `MatchPrefix("/")` → 交给 `command.Manager`
- 其它 → 交给默认处理器（例如 AI、FAQ、兜底提示）

## 进一步阅读

- 架构总览：`docs/architecture/overview.md`
- 企业微信案例的数据流：`docs/architecture/dataflow.md`（wecom 仅作为案例）
