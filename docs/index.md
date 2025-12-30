# 文档索引

更新时间：2025-12-16

本目录提供 **系统性说明**（概念/架构/使用指南）与 **代码功能说明**（自动生成的 API Reference）。

## 推荐阅读（Command 优先）

1) 先读：`docs/overview.md`（定位与边界）  
2) 再读：`docs/concepts/command.md`（指令系统的核心抽象）  
3) 上手：`docs/guides/command-quickstart.md`（新增一个命令并执行）  
4) 进阶：`docs/guides/command-advanced.md`（上下文、三种回复模式、Responder）  
5) 案例：`docs/cases/wecom.md`（企业微信接入，仅作为案例）  

## 目录导航

- 概览：`docs/overview.md`
- 概念：`docs/concepts/command.md` · `docs/concepts/pipeline.md`
- 指南：`docs/guides/command-quickstart.md` · `docs/guides/command-advanced.md`
- 架构：`docs/architecture/overview.md` · `docs/architecture/dataflow.md`
- 案例：`docs/cases/wecom.md`
- 附录（企业微信官方资料）：`docs/appendix/wecom-official/index.md`

## API Reference（自动生成）

`docs/reference/` 为自动生成目录，请勿手动编辑：

- `docs/reference/index.md`

本地生成方式：

```bash
scripts/generate-docs.sh          # 生成文档
scripts/generate-docs.sh --check  # 仅验证是否最新
```

## 社区与贡献

- 贡献指南：`CONTRIBUTING.md`
- 安全披露：`SECURITY.md`
- 变更日志：`CHANGELOG.md`
