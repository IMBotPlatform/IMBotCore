# Conventions

## Boundary Discipline

- `pkg/botcore` 放平台无关抽象，不放平台/产品专属逻辑
- `pkg/platform/*` 负责适配，不负责产品编排
- `pkg/command` 负责命令执行语义，不负责具体业务命令实现

## Documentation Sync

- `docs/reference/**` 不是手工真源，脚本与 CI 会重建它
- 公开 API 变化后优先执行 `scripts/generate-docs.sh --check`

## Testing Bias

- 行为变化优先用包级单元测试固定
- 改动 `pkg/platform/wecom` 时同时关注上游 `bot-protocol-wecom`

## Known Drift

- `README.md` 仍使用 `FirstSnapshot` 表述
- 当前代码导出名是 `RequestSnapshot`
- 处理该仓库时以 `pkg/botcore/request.go` 为事实源
