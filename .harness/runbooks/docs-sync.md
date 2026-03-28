# Runbook: Docs Sync

## Goal

在改动公开 API 后，避免 `docs/reference/**` 与代码漂移。

## Observed Mechanism

- 本地脚本：`scripts/generate-docs.sh`
- CI：`.github/workflows/docs.yml`
- CI 会在 `main` 分支相关路径变更时生成并提交 `docs/reference`

## Practical Sequence

1. 改动公开包
2. 运行 `scripts/generate-docs.sh --check`
3. 如需更新 reference，再运行 `scripts/generate-docs.sh`

## Trigger Paths

- `pkg/**`
- `go.mod`
- `go.sum`
- `scripts/generate-docs.sh`
