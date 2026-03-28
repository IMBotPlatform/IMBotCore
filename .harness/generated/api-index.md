# Generated API Index

## Package Inventory

- `github.com/IMBotPlatform/IMBotCore/pkg/botcore`
- `github.com/IMBotPlatform/IMBotCore/pkg/callback`
- `github.com/IMBotPlatform/IMBotCore/pkg/command`
- `github.com/IMBotPlatform/IMBotCore/pkg/container`
- `github.com/IMBotPlatform/IMBotCore/pkg/platform/wecom`
- `github.com/IMBotPlatform/IMBotCore/pkg/scheduler`
- `github.com/IMBotPlatform/IMBotCore/pkg/workspace`

## Notable Exported Types

- `RequestSnapshot`
- `StreamChunk`
- `PipelineContext`
- `Chain`
- `ExecutionContext`
- `Manager`
- `Scheduler`
- `Task`
- `Workspace`
- `FSCallback`
- `DockerRunner`
- `Bot` in `pkg/platform/wecom`

## Notable Exported Constructors / Helpers

- `botcore.NewChain`
- `botcore.MatchPrefix`
- `command.NewManager`
- `command.WithExecutionContext`
- `scheduler.New`
- `workspace.New`
- `workspace.DefaultConfig`
- `callback.NewFSCallback`
- `container.NewDockerRunner`
- `container.NewMountValidator`
- `platform/wecom.NewBot`
- `platform/wecom.NewPipelineAdapter`
