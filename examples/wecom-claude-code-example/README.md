# 企业微信 + Claude Code（GLM）示例

该示例展示如何用 IMBotCore 完成企业微信回调接入、命令路由，并通过 LLMClaudeCode 调用 Claude Code CLI（GLM Anthropic 兼容接口）实现流式输出。

## 依赖

- Go 1.24+
- 已安装 `claude` CLI（Claude Code），并确保可在 PATH 中找到
- 企业微信回调配置为 `http://<host>:8080/callback/command`
- 环境变量：
  - `WECOM_TOKEN`
  - `WECOM_ENCODING_AES_KEY`
  - `WECOM_CORP_ID`
  - `ANTHROPIC_AUTH_TOKEN`
  - `LISTEN_ADDR`（可选，默认 `:8080`）

> 说明：GLM 相关参数已在代码中固定，参考 `LLMClaudeCode/pkg/llm_glm_test.go`。

## 运行

```bash
export WECOM_TOKEN=...
export WECOM_ENCODING_AES_KEY=...
export WECOM_CORP_ID=...
export ANTHROPIC_AUTH_TOKEN=...

go run ./examples/wecom-claude-code-example
```

## 演示命令

- `/ping` -> 健康检查
- `/echo <text>` -> 回显输入
- `/ai <prompt>` -> 直接调用 LLM 流式输出

非命令消息（不以 `/` 开头）会走默认 AI 路由。
