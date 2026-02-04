# 删除 ConversationStore 体系（含 MemoryStore / Values / ConversationKey）设计

日期：2026-02-04

## 背景
用户要求：删除 MemoryStore 实现，并进一步删除 ConversationStore 接口；同时明确不再需要 ExecutionContext.Values 与 ConversationKey。

## 目标
- 移除 store 体系：ConversationStore / MemoryStore / 相关注入逻辑。
- 执行期上下文仅保留 RequestSnapshot、Responder、sendSignal。
- 最小化修改范围，更新示例与文档，避免 API 残留。

## 方案概览（Before → After）

```
Before
Manager.NewManager(factory, store)
  -> ExecutionContext { Values, Store, Responder, sendSignal }
  -> store.Load(conversationKey)

After
Manager.NewManager(factory)
  -> ExecutionContext { Responder, sendSignal }
  (无 store，无 Values，无 conversationKey)
```

## 关键修改点
1. 删除 `pkg/command/store.go`（MemoryStore）。
2. 修改 `pkg/command/context.go`：
   - 删除 ConversationStore 接口
   - 删除 ExecutionContext.Values 字段
   - 删除 ConversationKey()
3. 修改 `pkg/command/manager.go`：
   - 删除 Manager.store 字段
   - NewManager 签名改为 `NewManager(factory CommandFactory, opts ...ManagerOption)`
   - 删除 store.Load 逻辑
4. 更新文档与示例，移除 NewMemoryStore / ConversationStore 相关描述。

## 数据流与错误处理
- `Manager.Trigger` 仍负责解析命令、构建 Cobra 树、注入 ExecutionContext。
- `sendSignal` 仍用于当前请求的 StreamChunk 终止/静默信号。
- `Responder` 仍用于平台主动消息推送，与 sendSignal 职责区分。
- 删除 store.Load 后不再出现“上下文加载失败”日志路径。

## 测试建议
- `go test ./...`

