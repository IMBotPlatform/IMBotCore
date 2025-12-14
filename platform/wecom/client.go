package wecom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client 封装企业微信主动回复功能。
type Client struct {
	httpClient *http.Client
}

// NewClient 创建一个新的 Client。
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Send 向指定的 response_url 发送主动回复消息。
// 对应文档：7_加解密说明.md - 如何主动回复消息
// 注意：response_url 有效期为 1 小时，且每个 url 仅可调用一次。
func (c *Client) Send(responseURL string, msg interface{}) error {
	if responseURL == "" {
		return fmt.Errorf("response_url is empty")
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, responseURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("wecom api error: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	// 企业微信 API 通常返回 JSON，包含 errcode。这里简单检查 status code。
	// 如果需要更严格的错误检查，可以解析 respBody 中的 errcode。
	return nil
}

// MarkdownMessage 主动回复 Markdown 消息结构
type MarkdownMessage struct {
	MsgType  string          `json:"msgtype"` // markdown
	Markdown MarkdownPayload `json:"markdown"`
}

type MarkdownPayload struct {
	Content  string        `json:"content"`
	Feedback *FeedbackInfo `json:"feedback,omitempty"`
}

// SendMarkdown 发送 Markdown 消息
func (c *Client) SendMarkdown(responseURL, content string) error {
	msg := MarkdownMessage{
		MsgType: "markdown",
		Markdown: MarkdownPayload{
			Content: content,
		},
	}
	return c.Send(responseURL, msg)
}

// SendTemplateCard 发送模板卡片消息
func (c *Client) SendTemplateCard(responseURL string, card interface{}) error {
	typedCard, ok := card.(*TemplateCard)
	if !ok {
		return fmt.Errorf("invalid card type: expected *TemplateCard, got %T", card)
	}
	msg := TemplateCardMessage{
		MsgType:      "template_card",
		TemplateCard: typedCard,
	}
	return c.Send(responseURL, msg)
}
