# IMBotCore 概览

更新时间：2025-12-14

`IMBotCore` 是 `IMBotPlatform` 组织下的核心可复用包集合，目标是让你用**最小成本**把“聊天输入”变成“可执行的结构化指令（Command）”，并支持流式输出与上下文。

## 设计目标（第一性原理）

在机器人/助手类系统里，最难稳定的部分通常不是“接入某个平台”，而是：

1) **把输入统一成可路由的结构**（Update）  
2) **把结构化任务稳定执行**（Command）  
3) **把输出可靠交付**（Stream / Payload / NoResponse）  

所以 `IMBotCore` 优先解决 “Command 指令系统” 与 “流水线路由”，平台接入（例如企业微信）只作为案例与参考实现。

## 包结构

- `pkg/command`：命令系统（基于 Cobra），包含解析、执行、上下文、三种回复模式（文本流 / Payload / NoResponse）。
- `pkg/botcore`：基础抽象（Update / StreamChunk / PipelineInvoker / Chain 路由）。
- `pkg/platform/wecom`：企业微信接入案例（Crypt、Bot、Adapter、Emitter 等）。

## 你需要提供什么

`IMBotCore` 是“库”，不绑定你的 HTTP 框架/部署方式。你通常需要在你的服务中提供：

- 一个 **平台接入层**（或复用 `pkg/platform/wecom`）
- 一个 **Cobra 命令树工厂**（`func() *cobra.Command`）
- 一个 **Pipeline 组合**（例如 `botcore.Chain`：`/` 前缀走 command，其它走默认处理器）

## 下一步

- 直接上手：`docs/guides/command-quickstart.md`
- 理解核心抽象：`docs/concepts/command.md` · `docs/concepts/pipeline.md`
