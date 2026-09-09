package wecom

import (
	"errors"
	wecomproto "github.com/IMBotPlatform/bot-protocol-wecom/pkg/wecom"
)

// The destination comes only from the SDK callback; a supplied URL cannot retarget it.
type longConnResponser struct {
	bot      *wecomproto.LongConnBot
	chatID   string
	chatType wecomproto.LongConnChatType
}

func newLongConnResponser(ctx wecomproto.Context) *longConnResponser {
	r := &longConnResponser{bot: ctx.LongConn}
	if ctx.Message == nil {
		return r
	}
	switch ctx.Message.ChatType {
	case "single":
		r.chatID = ctx.Message.From.UserID
		r.chatType = wecomproto.LongConnChatTypeSingle
	case "group", "chatroom":
		r.chatID = ctx.Message.ChatID
		r.chatType = wecomproto.LongConnChatTypeGroup
	}
	return r
}
func (r *longConnResponser) SendMarkdown(content string) error {
	if r.bot == nil || r.chatID == "" {
		return errors.New("wecom conversation unavailable")
	}
	return r.bot.SendMarkdownWithChatType(r.chatID, r.chatType, content)
}
func (r *longConnResponser) ResponseMarkdown(_ string, content string) error {
	return r.SendMarkdown(content)
}
func (r *longConnResponser) ResponseTemplateCard(_ string, card any) error {
	c, ok := card.(*wecomproto.TemplateCard)
	if !ok || c == nil || r.bot == nil || r.chatID == "" {
		return errors.New("wecom card or conversation unavailable")
	}
	return r.bot.SendTemplateCardWithChatType(r.chatID, r.chatType, c)
}
func (r *longConnResponser) Response(_ string, msg any) error {
	switch v := msg.(type) {
	case string:
		return r.SendMarkdown(v)
	case wecomproto.MarkdownMessage:
		return r.SendMarkdown(v.Markdown.Content)
	case *wecomproto.MarkdownMessage:
		if v != nil {
			return r.SendMarkdown(v.Markdown.Content)
		}
	case *wecomproto.TemplateCard:
		return r.ResponseTemplateCard("", v)
	case wecomproto.TemplateCardMessage:
		return r.ResponseTemplateCard("", v.TemplateCard)
	case *wecomproto.TemplateCardMessage:
		if v != nil {
			return r.ResponseTemplateCard("", v.TemplateCard)
		}
	}
	return errors.New("unsupported proactive wecom message")
}
