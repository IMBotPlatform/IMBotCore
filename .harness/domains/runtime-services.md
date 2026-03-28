# Domain: Runtime Services

## Responsibility

描述 `scheduler`、`workspace`、`callback`、`container` 等运行期能力。

## Key Concepts

- `Scheduler`: SQLite-backed task scheduler
- `Workspace`: chat/session scoped filesystem isolation
- `FSCallback`: file-based IPC bridge
- `DockerRunner` / `MountValidator`: container execution helpers

## Main Flows

- 调度器周期轮询到期任务并回调 `TaskHandler`
- callback 通过文件目录交换消息、任务和审批
- workspace 为消息会话派生稳定路径

## Important Constraints

- 文件路径和挂载安全策略不应放宽为任意路径
- callback IPC 目录结构属于运行期约定

## Evidence

- `pkg/scheduler/scheduler.go`
- `pkg/scheduler/sqlite_scheduler.go`
- `pkg/callback/fs_callback.go`
- `pkg/workspace/workspace.go`
- `pkg/container/security.go`
- `pkg/container/docker.go`
