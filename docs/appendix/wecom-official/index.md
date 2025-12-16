# 附录：企业微信官方资料（仅引用）

更新时间：2025-12-14

本目录用于收纳与企业微信机器人接入相关的 **官方说明/存量资料**，便于查阅。

## 维护策略

- 仅做引用与归档：**不承诺持续同步更新**
- 若与官方最新文档有差异：**以官方为准**

## 资料索引

### 实现原理（基于 IMBotCore 案例）

- `docs/appendix/wecom-official/streaming.md`：流式输出实现原理
- `docs/appendix/wecom-official/interaction.md`：交互开发指南（消息类型、回复模式、事件路由）

### 官方文档（存量资料）

- `docs/appendix/wecom-official/wecom_ai_bot/1_概述.md`
- `docs/appendix/wecom-official/wecom_ai_bot/2_接收消息.md`
- `docs/appendix/wecom-official/wecom_ai_bot/3_接收事件.md`
- `docs/appendix/wecom-official/wecom_ai_bot/4_被动回复消息.md`
- `docs/appendix/wecom-official/wecom_ai_bot/5_模版卡片类型.md`
- `docs/appendix/wecom-official/wecom_ai_bot/6_回调和回复的加解密方案.md`
- `docs/appendix/wecom-official/wecom_ai_bot/7_加解密说明.md`

## 与 IMBotCore 的关系

`IMBotCore` 内的企业微信接入案例实现位于：

- `pkg/platform/wecom`

建议阅读路径：

1) 先理解 `IMBotCore` 的 Command 系统：`docs/concepts/command.md`
2) 再看企业微信接入案例：`docs/cases/wecom.md`
3) 最后按需查阅本附录中的官方资料

