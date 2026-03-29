# Project Harness

## Purpose

这个 harness 负责把 `IMBotCore` 现有文档体系压缩成 agent 可快速读取的入口图。

## Reading Order

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
