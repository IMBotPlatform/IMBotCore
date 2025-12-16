# 贡献指南（CONTRIBUTING）

感谢你对 `IMBotCore` 的兴趣与贡献意愿！

本仓库以“**最小可复用核心库**”为目标，优先保证抽象清晰、行为稳定、文档可追溯；因此更倾向于**小而聚焦**的改动。

## 开发环境

- Go：建议使用 `go.mod` 声明的 Go 版本或更高版本
- 平台：macOS / Linux / Windows 均可（仅要求 Go 工具链可用）

## 本地开发

```bash
go test ./...
```

## 文档（很重要）

手写文档在 `docs/`；API Reference 位于 `docs/reference/`，由脚本自动生成，请勿手工编辑生成产物。

```bash
scripts/generate-docs.sh          # 生成 docs/reference
scripts/generate-docs.sh --check  # CI/本地校验是否最新
```

## 提交与 PR 规范

- **保持最小改动面**：一次 PR 尽量只做一件事（一个问题/一个特性/一次重构）。
- **补齐验证**：涉及行为变化时，优先补充或更新测试（`go test ./...` 可验证）。
- **同步文档**：新增能力/修改接口时，请同步更新 `README.md` 或 `docs/` 对应说明。
- **生成文档**：如果你改了 `pkg/**` 的导出 API，请运行 `scripts/generate-docs.sh` 并提交生成结果。

## 报告问题（Issues）

提交 Issue 时建议包含：

- 复现步骤（最小可复现）
- 期望行为 vs 实际行为
- 版本信息（Go 版本、依赖版本、相关配置）
- 相关日志/报错（请脱敏）

