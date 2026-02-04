# ExecutionContext Send 接口内聚设计

日期：2026-02-04

## 背景
当前 `ExecutionContext` 通过 `Responder()` 暴露主动推送能力，且 `Send/SendMarkdown/SendTemplateCard` 由外部显式传入 `response_url`。同时，终止信号通过 `sender` 回调分散在 `Manager` 内部实现。

## 目标
- 去除 `ExecutionContext.responder` 与 `Responder` 接口。
- 将 `Send/SendMarkdown/SendTemplateCard` 迁移到 `ExecutionContext`，并统一使用 `RequestSnapshot.ResponseURL`。
- 去除 `ExecutionContext.sender`，改为直接持有输出通道 `ch`，并将终止信号的 `sync.Once` 逻辑内聚到 `ExecutionContext`。
- 保持最小改动范围与平台无关性（发送实现由 Manager 注入函数）。

## 方案总览（ASCII）
```
CommandManager.Trigger
  |
  +-- out chan StreamChunk
  +-- ExecutionContext
      - RequestSnapshot (含 ResponseURL)
      - ch
      - signalOnce
      - send/sendMarkdown/sendTemplateCard (函数注入)
  |
  +-- Cobra handler
      - cmd.Print* -> StreamWriter -> out (非 Final)
      - ctx.SendPayload / SendNoResponse -> ctx.sendSignal (Final once)
      - ctx.Send* -> 使用 ResponseURL 主动推送
  |
  +-- manager 兜底：ctx.sendSignal(空 Final)
```

## 关键调整点
1. `ExecutionContext`：新增 `Send/SendMarkdown/SendTemplateCard`，移除 `Responder()` 与 `responder` 字段。
2. `ExecutionContext`：新增 `ch` + `signalOnce`，内聚 `sendSignal` 逻辑，替代 `sender` 回调。
3. `Manager`：移除 `WithResponder`，改为注入发送函数（`send/sendMarkdown/sendTemplateCard`）。
4. 文档：更新 `Responder` 相关描述与示例代码。

## 错误与边界
- `Send*` 未注入发送函数时返回明确错误。
- `RequestSnapshot.ResponseURL` 为空时返回明确错误。
- 终止信号使用 `sync.Once` 保证只发送一次。

## 影响范围
- 代码：`pkg/command/context.go`, `pkg/command/manager.go`
- 文档：`docs/guides/command-advanced.md`, `docs/concepts/command.md`, `docs/appendix/wecom-official/interaction.md`, `docs/architecture/dataflow.md`, `docs/index.md`, `docs/reference/command.md` 等

## 测试建议
- `go test ./...`
