# Generated Module Map

| Path | Kind | Responsibility |
| --- | --- | --- |
| `pkg/botcore/` | public package | 平台无关请求/流式输出/责任链抽象 |
| `pkg/command/` | public package | Cobra 命令桥接、解析、上下文、流式写出 |
| `pkg/scheduler/` | public package | 任务模型、SQLite 调度器、回调机制 |
| `pkg/workspace/` | public package | 会话级文件系统工作空间 |
| `pkg/callback/` | public package | 文件系统 IPC callback |
| `pkg/container/` | public package | 容器执行与挂载安全校验 |
| `pkg/platform/wecom/` | public package | 企业微信适配层 |
| `docs/architecture/` | docs | 架构说明 |
| `docs/reference/` | generated docs | 自动生成 API reference |
| `examples/` | examples | WeCom + LLM 接入样例 |
