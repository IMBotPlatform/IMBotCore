package botcore

// Responser 定义主动发送能力的抽象接口。
// Parameters:
//   - responseURL: 平台回调中提供的 response_url
//   - msg/content/card: 待发送内容
//
// Returns:
//   - error: 发送失败时返回
//
// 该接口用于将平台实现（如 wecom.Bot）注入到 Manager 中，避免构造期循环依赖。
type Responser interface {
	Response(responseURL string, msg any) error
	ResponseMarkdown(responseURL, content string) error
	ResponseTemplateCard(responseURL string, card any) error
}

// 注意：Responser 仅定义能力抽象，具体注入请使用 (*Manager).WithResponser 方法。
