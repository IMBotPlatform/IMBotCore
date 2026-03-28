# Architecture

## Scope

本文件描述 `IMBotCore` 当前公开模块与依赖方向。

## System Shape

Observed fact:

- 仓库中心是 `pkg/botcore` 与 `pkg/command`
- 运行期配套能力分布在 `pkg/scheduler`、`pkg/workspace`、`pkg/callback`、`pkg/container`
- WeCom 平台适配位于 `pkg/platform/wecom`
- `docs/reference/**` 由脚本和 GitHub Actions 自动生成

## Major Modules

- `pkg/botcore`: `RequestSnapshot`、`StreamChunk`、`Chain`、`PipelineInvoker`
- `pkg/command`: Cobra 命令树工厂、解析器、执行上下文、流式写出
- `pkg/scheduler`: 任务模型、SQLite 调度器、到期任务回调
- `pkg/workspace`: 会话级文件系统工作空间
- `pkg/callback`: 基于文件系统的 IPC 回调桥
- `pkg/container`: 容器运行与挂载校验
- `pkg/platform/wecom`: 基于 `bot-protocol-wecom` 的适配层

## Dependency Directions

```text
botcore -> command / workspace / callback / platform adapters
platform/wecom -> bot-protocol-wecom
product repos -> IMBotCore public packages
```

Observed fact:

- `pkg/platform/wecom` 直接依赖 `bot-protocol-wecom`
- `wechataibot` 直接依赖本仓库多个包

## Key Flows

### Command Flow

```text
RequestSnapshot
  -> command.Parser
  -> command.Manager
  -> Cobra command tree
  -> StreamWriter / ExecutionContext
```

### Platform Flow

```text
WeCom SDK callback
  -> platform/wecom adapter
  -> botcore.PipelineInvoker
```

### Runtime Services

```text
workspace: isolated filesystem per chat/session
scheduler: SQLite-backed task loop
callback: file-based IPC for messages / tasks / approvals
```

## High-Risk Areas

- 公开抽象改名或字段变更
- 调度器任务语义变化
- workspace 路径约束与清理策略变化
- WeCom 适配层与上游 SDK 契约漂移

## Evidence

- `pkg/botcore/chain.go`
- `pkg/command/manager.go`
- `pkg/scheduler/scheduler.go`
- `pkg/workspace/workspace.go`
- `pkg/callback/fs_callback.go`
- `pkg/platform/wecom/wecom.go`
- `docs/architecture/overview.md`

## Open Questions

- `README.md` 与当前导出类型命名存在轻微漂移，需以代码为准
