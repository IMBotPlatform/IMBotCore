package botcore

// Bot 抽象首包快照构建与响应编码能力。
type Bot interface {
	// BuildFirstSnapshot 生成首包快照。
	BuildFirstSnapshot(raw any) (RequestSnapshot, error)

	// BuildReply 将流式片段编码为平台响应。
	BuildReply(firstSnapshot RequestSnapshot, chunk StreamChunk) (any, error)

	// Response 向指定的 response_url 发送主动回复消息。
	Response(responseURL string, msg any) error

	// ResponseMarkdown 发送 Markdown 消息。
	ResponseMarkdown(responseURL, content string) error

	// ResponseTemplateCard 发送模板卡片消息。
	ResponseTemplateCard(responseURL string, card any) error
}
