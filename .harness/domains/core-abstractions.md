# Domain: Core Abstractions

## Responsibility

定义平台无关的请求、流式输出、路由和命令执行抽象。

## Non-responsibility

- 不负责企业微信协议低层细节
- 不负责产品级 skill、部署或业务流程

## Key Concepts

- `RequestSnapshot`
- `StreamChunk`
- `PipelineInvoker`
- `Chain`
- `ExecutionContext`
- `Manager`

## Main Flows

- 文本请求进入 `Parser`
- 命令文本交给 `Manager` 构造 Cobra 命令树
- 非命令路由由上层 `Chain` 决定交给哪个 handler

## Important Constraints

- 抽象应保持平台无关
- 命令系统输出仍需落回统一 `StreamChunk`

## Evidence

- `pkg/botcore/chain.go`
- `pkg/botcore/pipeline.go`
- `pkg/command/manager.go`
- `pkg/command/context.go`
