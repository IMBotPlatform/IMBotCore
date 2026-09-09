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
- `StreamChunk.Replace=true` 表示完整文本替换，默认仍为追加；不能同时携带 `Payload`。`pkg/platform/wecom/adapter.go` 透传该字段，依赖的 SDK 必须包含对应 `Chunk.Replace` 契约。
- 交付前使用 `GOWORK=off go test -mod=mod ./...` 验证声明的模块版本；本机 vendor 或 workspace 替换不能证明独立 checkout 可编译。

## Evidence

- `pkg/botcore/chain.go`
- `pkg/botcore/pipeline.go`
- `pkg/command/manager.go`
- `pkg/command/context.go`
