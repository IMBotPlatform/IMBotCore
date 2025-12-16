# 企业微信机器人流式输出实现原理

本文档详细说明了如何利用企业微信的流式接口（`MsgType="stream"`）实现打字机效果的消息回复。核心机制包括：基于轮询的刷新机制、增量生产与全量叠加的状态管理。

> 本文档为存量资料，仅供参考。若与官方最新文档有差异，请以官方为准。

## 1. 整体架构与流程

企业微信的流式输出并非标准的 WebSocket 或 HTTP Chunked 响应，而是基于**"首包 + 轮询刷新"**的机制。

### 1.1 交互时序图

```ascii
+--------------------------------------------------------------------------------------------------+
|                                       流式输出架构                                                 |
+--------------------------------------------------------------------------------------------------+

                                         HTTP Request (Callback)
                                                    |
                                                    v
                                          [ pkg/platform/wecom/bot.go ]
                                              ServeHTTP
                                                    |
+-------------------------------------------------------+------------------------------------------+
| CASE 1: 首次消息 (MsgType != "stream")                  | CASE 2: 刷新请求 (MsgType == "stream")       |
+-------------------------------------------------------+------------------------------------------+
|                                                       |                                          |
| 1. Initial()                                          | 1. Refresh()                             |
|    |                                                  |    |                                     |
|    +--> [SessionManager] Create Session (StreamID)    |    +--> [SessionManager] Consume(StreamID) |
|    |                                                  |         (阻塞等待，直到 Timeout 或有数据)     |
|    +--> 启动 Goroutine: drainPipeline() --------------|--------+                                 |
|    |      |                                           |        |                                 |
|    |      v                                           |        v                                 |
|    |    [ pkg/botcore/router.go ]                     |    (获取到 Chunk)                          |
|    |      Trigger()                                   |        |                                 |
|    |      /      \                                    |    +---+---+                             |
|    |     /        \                                   |    | IsFinal?                            |
|    |   (Command)  (AI Chat)                           |    +---+---+                             |
|    |     /          \                                 |      |   |                               |
|    |    v            v                                |      |   +-- Yes --> [MarkFinished]      |
|    | [Manager]    [AIService]                         |      |                                   |
|    |    |            |                                |      v                                   |
|    |    | (Run)      | (Stream)                       |  [Encrypt Reply]                         |
|    |    v            v                                |      |                                   |
|    | [StreamWriter]  |                                |      v                                   |
|    | (io.Writer)     |                                |  HTTP Response                           |
|    |    |            |                                |  (返回给企业微信)                           |
|    +----+------------+                                |                                          |
|         |                                             |                                          |
|         v                                             |                                          |
|    chan StreamChunk                                   |                                          |
|         |                                             |                                          |
|         | <---- (drainPipeline 消费)                   |                                          |
|         v                                             |                                          |
|    [SessionManager] Publish(Chunk) -------------------+                                          |
|                                                       |                                          |
| 2. 返回空包 (ACK)                                      |                                          |
|    告诉企业微信 "已收到，请开始轮询"                       |                                          |
+-------------------------------------------------------+------------------------------------------+
```

### 1.2 核心步骤解析

1.  **接收消息 (`Initial`)**:
    *   用户发送消息，Bot 收到 HTTP POST。
    *   `Initial` 方法创建会话 (`Session`)，生成唯一 `StreamID`。
    *   **异步启动**业务逻辑（`drainPipeline`），开始生产数据。
    *   主线程立即返回一个空的流式响应包，通知企业微信："已进入流式模式，请开始轮询"。

2.  **业务处理 (`Pipeline`)**:
    *   `Router` 根据前缀分发任务（AI 对话或命令执行）。
    *   **Command**: 标准输出被劫持到 `StreamWriter`。
    *   **AI**: 大模型生成的 Token 被推送到 Channel。
    *   所有输出统一封装为 `StreamChunk`（含 `Content` 和 `IsFinal`）。

3.  **数据中转 (`SessionManager`)**:
    *   `drainPipeline` 从业务层读取增量片段，调用 `Publish` 存入 `SessionManager`。

4.  **流式回传 (`Refresh`)**:
    *   企业微信收到首包后，发起 `MsgType="stream"` 的请求。
    *   Bot 调用 `Consume` **阻塞等待**新数据。
    *   一旦有数据（或超时），立即返回当前**最新的全量内容**。
    *   循环此过程，直到返回 `IsFinal=true`。

## 2. 数据叠加与状态管理

为了适应企业微信的轮询机制，并解决网络抖动导致的"丢包"或"乱序"问题，Bot 内部采用**"增量生产，全量缓存，快照下发"**的策略。

### 2.1 数据流转原理

```ascii
+---------------------------------------------------------------------------------------------------------------+
|                                          数据流叠加原理                                                        |
+---------------------------------------------------------------------------------------------------------------+

[1. 生产者: Pipeline]             [2. 状态管理: SessionManager]                   [3. 消费者: HTTP Refresh]
(Command / AI Service)            (pkg/platform/wecom/session.go)                (pkg/platform/wecom/bot.go)

   Generate Incremental               Maintain Full State                            Poll & Response
         Chunks                           (Accumulation)                               (Snapshot)

           |                                    |                                            |
   +-------v-------+                 +----------v-----------+                                |
   | Chunk 1: "H"  | --------------> | Session.Accumulated  |                                |
   +---------------+    (Publish)    | Now: "H"             |                                |
                                     |                      |                                |
                                     | -> Enqueue Full: "H" |                                |
                                     +----------+-----------+                                |
                                                |                                  +---------v---------+
                                                | (Queue: ["H"])                   | WeCom Request #1  |
                                                |                                  | MsgType: stream   |
                                                v                                  +---------+---------+
                                     +----------+-----------+                                |
   +-------v-------+                 | Session.Accumulated  | <---- (Consume) ---------------+
   | Chunk 2: "el" | --------------> | Now: "Hel"           |       1. Take "H"              |
   +---------------+    (Publish)    |                      |       2. Return "H"            v
                                     | -> Enqueue Full:     |                           [ Response ]
                                     |    "Hel"             |                           Content: "H"
                                     +----------+-----------+                           IsFinal: false
                                                |
                                                | (Queue: ["Hel"])
                                                |
                                                v
                                     +----------+-----------+
   +-------v-------+                 | Session.Accumulated  |
   | Chunk 3: "lo" | --------------> | Now: "Hello"         |
   +---------------+    (Publish)    |                      |
                                     | -> Enqueue Full:     |
                                     |    "Hello"           |
                                     +----------+-----------+
                                                |
                                                | (Queue: ["Hel", "Hello"])
                                                |
                                                |                                  +---------v---------+
                                                |                                  | WeCom Request #2  |
                                                |                                  | MsgType: stream   |
                                                v                                  +---------+---------+
                                     +----------+-----------+                                |
   +-------v-------+                 | Session.Accumulated  | <---- (Consume) ---------------+
   | Chunk 4: "!"  | --------------> | Now: "Hello!"        |       1. Take "Hel"            |
   | IsFinal: true |    (Publish)    |                      |       2. Take "Hello" (Skip)   |
   +---------------+                 | -> Enqueue Full:     |       3. Return "Hello"        |
                                     |    "Hello!"          |          (Drain Queue)         v
                                     +----------+-----------+                           [ Response ]
                                                |                                       Content: "Hello"
                                                | (Queue: ["Hello!"])                   IsFinal: false *
                                                |                                       (*注: 还没取到!)
                                                v
                                                                                   +---------v---------+
                                                                                   | WeCom Request #3  |
                                                                                   +---------+---------+
                                                                                             |
                                                                    (Consume) <--------------+
                                                                    1. Take "Hello!"         |
                                                                    2. IsFinal=true          v
                                                                                        [ Response ]
                                                                                        Content: "Hello!"
                                                                                        IsFinal: true
```

### 2.2 关键机制说明

1.  **Publish (叠加)**:
    *   当业务层产生增量内容（如 "lo"）时，`SessionManager` 会将其追加到内存中的 `Accumulated` 字符串。
    *   然后，构造一个包含**完整内容**（"Hello"）的新 Chunk，放入发送队列。
    *   **意义**：保证队列中的每个元素都是独立的、完整的状态快照。

2.  **Consume (跳过)**:
    *   当 HTTP 请求到来时，`Consume` 方法会读取队列。
    *   **Drain 策略**：如果队列中积压了多个包（如 "Hel", "Hello", "Hello!"），它会循环读取直到队列为空，只返回最后一个。
    *   **意义**：避免客户端收到过期的中间状态，减少 HTTP 请求次数，提升用户体验。因为 "Hello!" 已经包含了 "Hel" 的所有信息。

3.  **Timeout & Fallback**:
    *   如果 `Refresh` 请求在 `Timeout` 时间内没有读到新数据，Bot 会返回当前的缓存快照（LastChunk），保持连接不中断。
    *   如果 `MsgType="stream"` 请求找不到对应的 Session（可能已过期或重启），系统会尝试从 `fallback` 缓存中查找是否有未发送的 `Final` 包，作为最后的兜底。

## 3. 关键组件代码映射

*   **`pkg/platform/wecom/bot.go`**:
    *   `Initial()`: 处理首包，启动 Pipeline。
    *   `Refresh()`: 处理轮询，调用 `Sessions.Consume()`。
*   **`pkg/platform/wecom/session.go`**:
    *   `Session`: 存储 `Accumulated` 文本和 `queue`。
    *   `Publish()`: 执行 `Accumulated += chunk` 并入队。
    *   `Consume()`: 执行队列排干（Drain）逻辑。
*   **`pkg/botcore/router.go`**:
    *   `Chain` + `AddRoute`：基于前缀路由到命令或 AI。
    *   `MatchPrefix("/")`：匹配命令前缀，未匹配走默认处理。
*   **`pkg/command/io.go`**:
    *   `StreamWriter`: 适配 `io.Writer`，让普通 CLI 工具的输出也能无缝转化为流式片段。

> 更多信息请参考 `docs/appendix/wecom-official/index.md`。
