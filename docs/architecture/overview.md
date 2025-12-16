# 架构总览

更新时间：2025-12-14

## 模块职责映射
- `pkg/command`：指令系统（解析 + Cobra 执行 + ExecutionContext + 流式输出）。
- `pkg/botcore`：平台无关的基础抽象（Update / StreamChunk / Chain 路由）。
- `pkg/ai`：可选的 LLM 能力封装（配置、SessionStore、流式 Chat）。
- `pkg/platform/wecom`：企业微信接入案例（验签/加解密、回调处理、流式会话管理）。

## 整体调用链
```
Platform Webhook / HTTP
        |
        v
   platform handler
 (e.g. wecom.Bot)
 [verify + decrypt + normalize]
        |
        v
    botcore.Chain
   |----------------------|
   |                      |
Prefix "/" route          Default route (optional)
   |                      |
command.Manager           your handler (optional)
   |                      |
Cobra commands            (e.g. ai.Service.Chat)
   |
StreamChunk (Content / Payload / NoResponse)
```

## 数据与状态
- 命令上下文：可用 `command.ConversationStore` 存取（默认实现：`command.MemoryStore`）。
- 对话历史：`pkg/ai` 提供 `FileStore`（JSONL），也可替换为 Redis/MySQL 等实现。
- 模型实例：`pkg/ai.Service` 会按需初始化并在进程内缓存（生命周期随进程）。

## 关键路由规则
- 以 `/` 开头：`botcore.MatchPrefix("/")` → `command.Manager` → 执行业务命令。
- 其它输入：交给你自定义的默认处理器（例如 AI、FAQ、兜底提示等）。

## 扩展点
- 新增命令：在 `CommandFactory` 中注册新的 Cobra 子命令即可。
- 新增路由：在 `botcore.Chain.AddRoute(...)` 增加规则（按顺序匹配）。
- 新增平台：实现平台接入层并输出 `botcore.Update`（或复用 `pkg/platform/wecom` 案例）。
- 可选 AI：实现 `command.LLMProvider` 并通过 `command.WithLLM(...)` 注入到 `Manager`。
