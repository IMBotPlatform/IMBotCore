# Command WithSender 设计

## 背景与目标
当前示例通过闭包捕获 `bot` 来注入主动发送能力，可用但依赖不够显式。目标是在 `command` 包提供 `WithSender(sender)`，让 `Manager` 显式依赖抽象发送器，实现更清晰的初始化与依赖关系，并保持最小改动。

约束：
- 不破坏已有 `WithSend/WithSendMarkdown/WithSendTemplateCard` 用法
- 修改范围尽量限定在 `command` 包与示例入口
- 行为可预测、错误显式

## 架构图（ASCII）
```
HTTP Callback
    |
   Bot (wecom) implements Sender
    |
  Chain
    |
 Manager (WithSender)
    |
ExecutionContext.Send*
```

## 核心接口设计
新增 `Sender` 抽象与 `WithSender` 选项：

```go
type Sender interface {
    Send(responseURL string, msg any) error
    SendMarkdown(responseURL, content string) error
    SendTemplateCard(responseURL string, card any) error
}

func WithSender(sender Sender) ManagerOption
```

语义：`WithSender` 设置三类发送函数。`ManagerOption` 按传入顺序应用，后续 option 覆盖前者；如需自定义发送，可在 `WithSender` 之后再传 `WithSend*`。

## 初始化流程（方案 A）
1. 创建 `chain`（默认处理器保持不变）
2. 创建 `bot`（注入 `chain`）
3. 创建 `manager`（传 `WithSender(bot)`）
4. `chain.AddRoute("command", ..., manager)`
5. 启动 HTTP

## 伪代码
```go
chain := botcore.NewChain(newAIHandler(llm))

bot, err := wecom.NewBot(..., chain)
if err != nil { ... }

manager := command.NewManager(
    func() *cobra.Command { return newRootCmd(llm) },
    command.WithSender(bot),
)

chain.AddRoute("command", botcore.MatchPrefix("/"), manager)
```

## 错误处理
- `sender == nil`：`WithSender` 内部直接返回错误或留空函数；`ExecutionContext.Send*` 调用时返回明确错误
- `response_url` 为空：沿用现有 `ExecutionContext` 错误

## 影响范围
- 新增接口与 option：`pkg/command`
- 示例调整初始化顺序：`examples/wecom-openai-example/main.go`、`examples/wecom-claude-code-example/main.go`
- 可选更新文档：`docs/reference/command.md`

## 测试建议
- 可选：新增轻量单测验证 `WithSender` 注入三类发送函数（不强制）
- 手工：运行示例并触发 `/ping` 与命令内主动发送路径
