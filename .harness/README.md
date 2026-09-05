# Project Harness

## Purpose

这个 harness 负责把 `IMBotCore` 现有文档体系压缩成 agent 可快速读取的入口图。

## 按任务读取

先遵守本 repo 的 AGENTS，再进入 `../pkg/command/`、`../pkg/scheduler/`、`../pkg/workspace/` 中与任务对应的实现；平台适配任务再读 `../pkg/platform/`。

验证或修改时选择 `generated/validation-index.yaml`；API/类型改变选择 contract/api index；完整架构与运行手册只在证据不足时展开。下列保留完整导航，不构成预读顺序：

1. `generated/repo-manifest.yaml`
2. `generated/module-map.md`
3. `generated/contract-index.yaml`
4. `generated/validation-index.yaml`
5. `evolution-policy.yaml`
6. `generated/api-index.md`
7. `architecture.md`
8. `domains/core-abstractions.md`
9. `domains/runtime-services.md`
10. `runbooks/local-validation.md`
11. `runbooks/docs-sync.md`

## Evidence Priority

1. `pkg/**`
2. `pkg/**/*_test.go`
3. `docs/architecture/**`
4. `docs/reference/**`
5. `README.md`
