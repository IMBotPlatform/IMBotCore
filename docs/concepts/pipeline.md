# Pipeline / Router 概念说明

更新时间：2025-12-14

本页解释 `pkg/botcore` 的核心抽象，用于把平台输入统一成 `Update`，再通过路由把它交给不同的处理器（例如 `command.Manager`）。

## 关键抽象

### 1) Update

`botcore.Update` 是平台无关的标准事件结构，包含：

- `Text`：文本内容（常用于命令匹配）
- `ChatID` / `SenderID`：会话与用户维度
- `Raw`：保留平台原始结构（必要时可深入）
- `Metadata`：扩展字段（例如 `platform`、`response_url` 等）

### 2) StreamChunk

`botcore.StreamChunk` 是处理器输出的统一格式：

- `Content`：文本增量（可多次发送）
- `Payload`：复杂对象（例如卡片/结构化响应）
- `IsFinal`：结束信号

### 3) PipelineInvoker

`PipelineInvoker` 是统一的“执行器”接口：

```go
Trigger(update Update, streamID string) <-chan StreamChunk
```

其中 `streamID` 用于标识一次流式会话（由上层平台接入层生成/管理）。

### 4) Chain（责任链路由）

`botcore.Chain` 提供一个最小路由器：

- 按顺序检查每条 `Route.Matcher`
- 命中则交给对应 `Handler`
- 否则交给 `defaultHandler`

最常见的策略：

- `MatchPrefix("/")` → 交给 `command.Manager`
- 其它 → 交给默认处理器（例如 AI、FAQ、兜底提示）

## 进一步阅读

- 架构总览：`docs/architecture/overview.md`
- 企业微信案例的数据流：`docs/architecture/dataflow.md`（wecom 仅作为案例）

