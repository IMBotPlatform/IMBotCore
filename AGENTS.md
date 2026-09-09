# AGENTS.md

## Mission / Scope

`IMBotCore` 是平台无关的 Bot Core 仓库。

- owns: `botcore` 抽象、命令系统、调度器、会话工作空间、文件 IPC callback、WeCom 平台适配
- not owns: 具体业务技能、产品部署、具体 Agent Runtime 的实现与产品选择策略

## Start Here

先按本文件的责任边界和源码地图选择入口；需要 Harness 导航时读 `.harness/README.md`。下列索引按任务选择：合同变化读 contract/api index，验证读 validation index，Harness 维护读 evolution policy；不要求逐个预读。


1. `.harness/README.md`
2. `.harness/generated/module-map.md`
3. `.harness/generated/repo-manifest.yaml`
4. `.harness/generated/contract-index.yaml`
5. `.harness/generated/validation-index.yaml`
6. `.harness/evolution-policy.yaml`
7. `.harness/generated/api-index.md`
8. `docs/architecture/overview.md`
9. `README.md`

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
- 流式 `Replace` 语义必须透传到平台 SDK；发布前核对 go.mod 中的 SDK 版本能够独立编译。
- `pkg/container` 必须保持 Runtime 中立；镜像、容器内工作/状态路径和环境变量白名单由调用方显式配置
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
