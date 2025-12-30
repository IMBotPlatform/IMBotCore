# 企业微信示例

该示例展示如何用 IMBotCore 完成企业微信回调接入、命令路由，并直接使用 langchaingo 做大模型流式输出。

## 依赖

- Go 1.24+
- 企业微信回调配置为 `http://<host>:8080/callback/command`
- 环境变量：
  - `WECOM_TOKEN`
  - `WECOM_ENCODING_AES_KEY`
  - `WECOM_CORP_ID`
  - `OPENAI_API_KEY`
  - `OPENAI_MODEL`（可选）
  - `OPENAI_BASE_URL`（可选）
  - `LISTEN_ADDR`（可选，默认 `:8080`）

## 运行

```bash
export WECOM_TOKEN=...
export WECOM_ENCODING_AES_KEY=...
export WECOM_CORP_ID=...
export OPENAI_API_KEY=...

go run ./examples/wecom-example
```

## 演示命令

- `/ping` -> 健康检查
- `/echo <text>` -> 回显输入
- `/ai <prompt>` -> 直接调用 LLM 流式输出

非命令消息（不以 `/` 开头）会走默认 AI 路由。
