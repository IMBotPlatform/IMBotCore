# 企业微信交互开发指南

本文档说明企业微信的多种交互模式，包括被动回复（流式/非流式）、主动消息推送以及各类事件处理。

> 本文档为存量资料，仅供参考。若与官方最新文档有差异，请以官方为准。

## 1. 支持的消息类型矩阵

### 1.1 接收消息 (Bot 接收)
| 类型 | 字段 (`MsgType`) | 对应结构体 | 说明 |
| :--- | :--- | :--- | :--- |
| **文本** | `text` | `TextPayload` | 用户发送的普通文本 |
| **图片** | `image` | `ImagePayload` | 用户发送的图片 (含 URL) |
| **语音** | `voice` | `VoicePayload` | 用户发送的语音 (含转录文本) |
| **文件** | `file` | `FilePayload` | 用户发送的文件 (含下载 URL) |
| **图文混排** | `mixed` | `MixedPayload` | 包含文本和图片的组合消息 |
| **引用消息** | (无独立类型) | `QuotePayload` | 嵌套在上述消息中的 `quote` 字段 |

### 1.2 接收事件 (Bot 接收)
| 事件场景 | `EventType` | 对应结构体 | 默认路由行为 |
| :--- | :--- | :--- | :--- |
| **进入会话** | `enter_chat` | `EnterChat` | 自动路由为 `/welcome` 指令 |
| **模板卡片交互** | `template_card_event` | `TemplateCardEvent` | 将 `event_key` 作为指令执行 (e.g. `/click`) |
| **用户反馈** | `feedback_event` | `FeedbackEvent` | 仅记录，需自定义处理 |

### 1.3 被动回复 (同步 HTTP 响应)
| 类型 | 字段 (`MsgType`) | 结构体 | 适用场景 |
| :--- | :--- | :--- | :--- |
| **流式文本** | `stream` | `StreamReply` | **核心功能**，AI 对话逐字输出 |
| **流式+图文** | `stream` | `StreamReply` | 流式结束 (`finish=true`) 时附带图片 |
| **普通文本** | `text` | `TextMessage` | 简单的非流式回复 (如指令报错) |
| **模板卡片** | `template_card` | `TemplateCardMessage` | 发送欢迎语、功能菜单 |
| **流式+卡片** | `stream_with_template_card` | `StreamWithTemplateCardMessage` | AI 回复的同时下发一张卡片 |
| **更新卡片** | `update_template_card` | `UpdateTemplateCardMessage` | 用户点击按钮后，更新原卡片状态 |

### 1.4 主动回复 (异步 HTTP 推送)
利用 `response_url` 在 1 小时内主动下发。

| 类型 | 字段 (`MsgType`) | 结构体 | 调用方式 |
| :--- | :--- | :--- | :--- |
| **Markdown** | `markdown` | `MarkdownMessage` | `responder.SendMarkdown(...)` |
| **模板卡片** | `template_card` | `TemplateCardMessage` | `responder.SendTemplateCard(...)` |

---

## 2. 三种回复模式 (开发核心)

在 Command 的 `Run` 方法中，你可以根据需求选择三种不同的回复模式。

### 2.1 模式一：流式文本回复 (默认)
最常用的模式。只要通过 `cmd.Printf` 或 `cmd.Println` 输出内容，系统就会自动建立流式会话，将内容逐字推送到企业微信气泡中。

**代码示例:** 
```go
func Run(cmd *cobra.Command, args []string) {
    cmd.Println("正在查询数据...") // 第一帧
    // ... 耗时操作
    cmd.Println("查询完成: 100条") // 后续帧
}
```

### 2.2 模式二：静默执行 (No Response)
当你不希望 Bot 在对话框产生任何气泡（连"正在生成"的转圈都不想要），而是想完全依赖**主动推送**（如发一张独立的卡片）时，使用此模式。

**代码示例:** 
```go
func Run(cmd *cobra.Command, args []string) {
    ctx := command.FromContext(cmd.Context())
    
    // 1. 标记静默：Bot 将直接返回 HTTP 200 OK 空包
    ctx.SetNoResponse()

    // 2. 使用 Responder 主动推送消息 (异步)
    responder := ctx.Responder()
    respURL := ctx.Update.Metadata["response_url"]
    responder.SendMarkdown(respURL, "# 异步通知\n任务已后台开始")
}
```

### 2.3 模式三：非流式对象回复 (Payload)
当你需要同步回复一个特殊对象（如一张卡片，或者更新卡片状态）而不是文本流时，使用此模式。这通常用于`enter_chat`欢迎语或按钮点击回调。

**代码示例:** 
```go
func Run(cmd *cobra.Command, args []string) {
    ctx := command.FromContext(cmd.Context())
    
    // 构造更新消息 (例如点击按钮后将按钮变为文本)
    msg := &wecom.UpdateTemplateCardMessage{
        ResponseType: "update_template_card",
        TemplateCard: &wecom.TemplateCard{
            TaskID: ctx.Update.Metadata["task_id"],
            MainTitle: &wecom.MainTitle{Title: "已批准"},
        },
    }
    
    // 设置 Payload：Bot 将序列化此对象并加密返回
    ctx.SetResponsePayload(msg)
}
```

---

## 3. 事件与路由机制

为了简化开发，系统底层会自动将特定的企业微信**事件**转换为**文本指令**，从而复用现有的 Command 命令系统。

### 3.1 事件映射表

| 原始事件类型 (`event_type`) | 转换逻辑 | 路由结果 (Text) | 推荐实现方式 |
| :--- | :--- | :--- | :--- |
| `enter_chat` (进入会话) | 固定映射 | `/welcome` | 注册名为 `welcome` 的 Command 用于发送欢迎语 |
| `template_card_event` (卡片交互) | 取 `event_key` | `event_key` 的值 | 将按钮 Key 设为 `/cmd` 形式，直接触发对应 Command |
| `feedback_event` (用户反馈) | (仅透传 Metadata) | (无特殊文本) | 需在中间件或自定义逻辑中处理 |

## 4. 开发场景示例

### 场景 A: 实现欢迎语
1.  注册命令 `Use: "welcome"`。
2.  在 `Run` 中使用 `ctx.SetResponsePayload(&wecom.TemplateCardMessage{...})` 返回一张欢迎卡片。

### 场景 B: 卡片按钮交互
1.  发送一张包含按钮的卡片，按钮 Key 设为 `/approve order_123`。
2.  用户点击按钮，企业微信推送 `template_card_event`。
3.  适配器将其转换为文本 `/approve order_123`。
4.  系统路由至 `approve` 命令。
5.  `approve` 命令使用 `ctx.SetResponsePayload(&wecom.UpdateTemplateCardMessage{...})` 更新原卡片状态。

> 更多信息请参考 `docs/appendix/wecom-official/index.md`。
