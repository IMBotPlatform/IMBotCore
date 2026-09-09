package wecom

import (
	"github.com/IMBotPlatform/IMBotCore/pkg/botcore"
	proto "github.com/IMBotPlatform/bot-protocol-wecom/pkg/wecom"
	"testing"
)

func TestLongConnSnapshotAndBoundDestination(t *testing.T) {
	for _, kind := range []string{"single", "group"} {
		t.Run(kind, func(t *testing.T) {
			bot, _ := proto.NewLongConnBot("fixture", "fixture", nil)
			msg := &proto.Message{MsgID: "message", ChatType: kind, From: proto.MessageSender{UserID: "sender"}, Text: &proto.TextPayload{Content: "/help"}, MsgType: "text"}
			if kind == "group" {
				msg.ChatID = "group-id"
			}
			p := botcore.PipelineFunc(func(pc botcore.PipelineContext) <-chan botcore.StreamChunk {
				if pc.Snapshot.ID != "message" || pc.Snapshot.Metadata["transport"] != "websocket" || pc.Snapshot.ResponseURL != "" || pc.Snapshot.Text != "/help" {
					t.Fatal("lost long-connection snapshot")
				}
				r, ok := pc.Responser.(*longConnResponser)
				if !ok {
					t.Fatal("HTTP responder installed")
				}
				if _, ok := pc.Responser.(botcore.ConversationSender); !ok {
					t.Fatal("no bound sender")
				}
				wantID, wantType := "sender", proto.LongConnChatTypeSingle
				if kind == "group" {
					wantID, wantType = "group-id", proto.LongConnChatTypeGroup
				}
				if r.chatID != wantID || r.chatType != wantType {
					t.Fatal("wrong destination")
				}
				out := make(chan botcore.StreamChunk, 1)
				out <- botcore.StreamChunk{Replace: true, Content: "help", IsFinal: true}
				close(out)
				return out
			})
			out := NewPipelineAdapter(p).Handle(proto.Context{Message: msg, RequestID: "request", LongConn: bot})
			c := <-out
			if !c.Replace || c.Content != "help" {
				t.Fatal("stream conversion lost replacement")
			}
			for range out {
			}
		})
	}
}
func TestMissingHTTPResponderReturnsErrors(t *testing.T) {
	r := &BotResponser{}
	if r.ResponseMarkdown("", "a") == nil || r.Response("", nil) == nil || r.ResponseTemplateCard("", nil) == nil {
		t.Fatal("nil responder reported success")
	}
}
func TestLongConnSnapshotRequestIDFallback(t *testing.T) {
	s := buildSnapshot(proto.Context{Message: &proto.Message{}, RequestID: "callback"})
	if s.ID != "callback" {
		t.Fatal("no fallback ID")
	}
	r := newLongConnResponser(proto.Context{Message: &proto.Message{ChatType: "unknown", ChatID: "attacker"}})
	if r.SendMarkdown("a") == nil || r.Response("", 42) == nil || r.ResponseTemplateCard("", nil) == nil {
		t.Fatal("invalid target/payload reported success")
	}
}
