# Runbook: Local Validation

## Default Chain

```bash
go test ./...
scripts/generate-docs.sh --check
```

## When To Re-run Full Suite

- 改动 `pkg/botcore/**`
- 改动 `pkg/command/**`
- 改动 `pkg/platform/wecom/**`
- 改动 `pkg/scheduler/**`
- 改动 `pkg/workspace/**`

## Known Caveat

Observed fact:

- 本地 `go list ./...` 报告了来自 `../bot-protocol-wecom` replace 目标的 `github.com/gorilla/websocket` `go.sum` 条目缺失

Practical rule:

- 如果验证前就出现 module 解析异常，先确认依赖状态，再判断是否为本次改动引入
