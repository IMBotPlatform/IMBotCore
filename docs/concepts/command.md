# Command 概念说明

更新时间：2025-12-14

本页解释 `pkg/command` 的核心抽象与它解决的问题：**把一段文本指令稳定地解析、路由、执行，并把输出转成流式结果**。

## 关键抽象

### 1) Parser（指令识别与拆词）

- 默认规则：以 `/` 开头才视为命令，例如 `/help`、`/ping`。
- `Parser.Parse(text)` 输出 `ParseResult`：是否命令、token 列表、参数原文等。

### 2) CommandFactory（命令树工厂）

在机器人/HTTP 场景中，**每个请求都要创建一棵全新的 Cobra 命令树**，避免并发时共享 Flag 状态导致串扰。

因此 `pkg/command` 使用：

- `type CommandFactory func() *cobra.Command`

### 3) Manager（命令执行器 / PipelineInvoker）

`Manager` 实现了 `botcore.PipelineInvoker`：

- 输入：`botcore.Update`（来自平台适配器）
- 输出：`<-chan botcore.StreamChunk`（流式片段）

它内部做了：

1) Parse：判断是不是命令
2) Build：调用 `CommandFactory()` 构建 root command
3) Redirect IO：把 `cmd.Println` 等输出重定向为 `StreamChunk`
4) Execute：执行 Cobra 命令
5) Final：确保最终发送 `IsFinal=true` 结束信号

### 4) ExecutionContext（命令执行上下文）

命令 handler 可以通过 `command.FromContext(cmd.Context())` 取到 `ExecutionContext`，用于：

- 获取本次 `Update`（例如 ChatID / SenderID / Metadata）
- 读写上下文键值（`Values` + `ConversationStore`）
- 可选主动推送（`Responder()`）
- 发送两类特殊信号：`SetResponsePayload`、`SetNoResponse`

## 三种输出语义（概念先行）

命令执行时，输出语义分为三类（具体示例见 `docs/guides/command-advanced.md`）：

1) **文本流**：直接 `cmd.Print*` 输出，转成 `StreamChunk.Content`（非 Final）
2) **Payload**：`ctx.SetResponsePayload(payload)`，转成 `StreamChunk.Payload`（Final）
3) **NoResponse**：`ctx.SetNoResponse()`，告知上层“不要产生被动回复”（Final）

## 进一步阅读

- 上手新增命令：`docs/guides/command-quickstart.md`
- 进阶：上下文/Responder/输出模式：`docs/guides/command-advanced.md`
