# Glossary

## RequestSnapshot

- Definition: 当前代码中的平台无关请求首包结构
- Evidence: `pkg/botcore/request.go`
- Common ambiguity: `README.md` 仍提到 `FirstSnapshot`

## StreamChunk

- Definition: 统一流式输出片段，带 `Content` / `Payload` / `IsFinal`
- Evidence: `pkg/botcore/pipeline.go`

## Chain

- Definition: 按顺序匹配路由并触发 `PipelineInvoker` 的责任链
- Evidence: `pkg/botcore/chain.go`

## ExecutionContext

- Definition: 命令执行时绑定请求快照、流式输出通道与可选 `Responser`
- Evidence: `pkg/command/context.go`

## Workspace

- Definition: 会话级文件系统工作空间抽象
- Evidence: `pkg/workspace/workspace.go`
