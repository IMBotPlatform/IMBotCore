# AGENTS.md

## Mission / Scope

`IMBotCore` 是平台无关的 Bot Core 仓库。

- owns: `botcore` 抽象、命令系统、调度器、会话工作空间、文件 IPC callback、WeCom 平台适配
- not owns: 具体业务技能、产品部署、Claude CLI 的底层执行实现

## Start Here

1. `.harness/README.md`
2. `.harness/generated/repo-manifest.yaml`
3. `.harness/generated/module-map.md`
4. `docs/architecture/overview.md`
5. `README.md`

## Source of Truth

- 代码：`pkg/**`
- 架构说明：`docs/architecture/overview.md`
- API 镜像：`docs/reference/**`
- 生成脚本：`scripts/generate-docs.sh`

## Important Directories

- `pkg/botcore/`
- `pkg/command/`
- `pkg/scheduler/`
- `pkg/workspace/`
- `pkg/callback/`
- `pkg/platform/wecom/`
- `pkg/container/`
- `docs/`

## Hard Constraints

- 不把 `wechataibot` 这种产品仓库的业务细节拉进 core 包
- 改动公开 API 后，同步关注 `docs/reference/` 的更新链
- 改动 `pkg/platform/wecom/`、`pkg/command/`、`pkg/workspace/` 后，需要考虑 `wechataibot` 兼容性

## Validation Expectations

- `go test ./...`
- `scripts/generate-docs.sh --check`

## High-Risk Areas

- `pkg/platform/wecom/`
- `pkg/command/`
- `pkg/workspace/`
- `pkg/scheduler/`
- `pkg/callback/`
