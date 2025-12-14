package wecom

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/IMBotPlatform/IMBotCore/pkg/botcore"
)

func TestCalcSignatureDeterministic(t *testing.T) {
	sig1 := calcSignature("token", "12345", "nonce", "cipher")
	sig2 := calcSignature("token", "12345", "nonce", "cipher")
	if sig1 != sig2 {
		t.Fatalf("signature mismatch: %s vs %s", sig1, sig2)
	}
}

func TestCryptEncryptDecryptRoundTrip(t *testing.T) {
	rawKey := bytes.Repeat([]byte{0x11}, 32)
	encodingKey := base64.StdEncoding.EncodeToString(rawKey)
	encodingKey = strings.TrimRight(encodingKey, "=")
	crypt, err := NewCrypt("token", encodingKey, "corpID")
	if err != nil {
		t.Fatalf("create crypt: %v", err)
	}
	payload := BuildStreamReply("stream-id", "hello", false)
	ts := "1700000000"
	resp, err := crypt.EncryptResponse(payload, ts, "nonce")
	if err != nil {
		t.Fatalf("encrypt reply: %v", err)
	}
	msg, err := crypt.DecryptMessage(resp.MsgSignature, resp.Timestamp, resp.Nonce, EncryptedRequest{Encrypt: resp.Encrypt})
	if err != nil {
		t.Fatalf("decrypt message: %v", err)
	}
	if msg.MsgType != "stream" {
		t.Fatalf("unexpected msgtype: %s", msg.MsgType)
	}
	if msg.Stream == nil || msg.Stream.ID != "stream-id" {
		t.Fatalf("unexpected stream payload: %#v", msg.Stream)
	}
}

func TestSessionManagerPublishConsume(t *testing.T) {
	mgr := NewSessionManager(50 * time.Millisecond)
	msg := &Message{MsgID: "mid", ChatID: "cid", From: MessageSender{UserID: "uid"}}
	session, isNew := mgr.CreateOrGet(msg)
	if !isNew {
		t.Fatalf("expected new session")
	}
	if session.StreamID == "" {
		t.Fatalf("empty stream id")
	}
	if _, ok := mgr.GetStreamIDByMsg("mid"); !ok {
		t.Fatalf("msg id not indexed")
	}

	// Publish two chunks rapidly
	mgr.Publish(session.StreamID, botcore.StreamChunk{Content: "chunk1"})
	mgr.Publish(session.StreamID, botcore.StreamChunk{Content: "final", IsFinal: true})

	// Consume with a small timeout - should drain both and return merged result
	chunk := mgr.Consume(session.StreamID, 10*time.Millisecond)
	if chunk == nil {
		t.Fatalf("expected chunk")
	}
	// Expect merged content "chunk1final"
	if chunk.Content != "chunk1final" {
		t.Fatalf("expected merged content 'chunk1final', got '%s'", chunk.Content)
	}
	if !chunk.IsFinal {
		t.Fatalf("expected merged chunk to be final")
	}

	time.Sleep(60 * time.Millisecond)
	mgr.Cleanup()
	if _, ok := mgr.GetStreamIDByMsg("mid"); ok {
		t.Fatalf("session not cleaned")
	}
}

func TestBotRefreshFallback(t *testing.T) {
	rawKey := bytes.Repeat([]byte{0x22}, 32)
	key := strings.TrimRight(base64.StdEncoding.EncodeToString(rawKey), "=")
	crypt, err := NewCrypt("token", key, "corpID")
	if err != nil {
		t.Fatalf("create crypt: %v", err)
	}
	mgr := NewSessionManager(time.Minute)
	bot := &Bot{Sessions: mgr, Crypto: crypt, Timeout: 5 * time.Millisecond, Adapter: MessageAdapter{}, Emitter: StreamEmitter{}}
	msg := &Message{MsgID: "mid", MsgType: "text", Text: &TextPayload{Content: "hi"}, From: MessageSender{UserID: "uid"}}
	ts := "1700000000"
	initialResp, err := bot.Initial(msg, ts, "nonce")
	if err != nil {
		t.Fatalf("initial: %v", err)
	}
	if initialResp.Encrypt == "" {
		t.Fatalf("empty encrypt in initial resp")
	}
	sessionID, ok := mgr.GetStreamIDByMsg("mid")
	if !ok {
		t.Fatalf("missing stream id")
	}
	bot.SetFinalMessage("mid", "done")
	refresh := &Message{MsgID: "mid", MsgType: "stream", Stream: &StreamPayload{ID: sessionID}}
	refreshResp, err := bot.Refresh(refresh, ts, "nonce")
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	msgPlain, err := crypt.DecryptMessage(refreshResp.MsgSignature, refreshResp.Timestamp, refreshResp.Nonce, EncryptedRequest{Encrypt: refreshResp.Encrypt})
	if err != nil {
		t.Fatalf("decrypt refresh: %v", err)
	}
	if msgPlain.Stream == nil || msgPlain.Stream.ID != sessionID {
		t.Fatalf("unexpected stream id: %#v", msgPlain.Stream)
	}
	if msgPlain.MsgType != "stream" {
		t.Fatalf("unexpected msgtype: %s", msgPlain.MsgType)
	}
	if msgPlain.Stream.Content != "done" {
		t.Fatalf("unexpected content: %s", msgPlain.Stream.Content)
	}
	if !msgPlain.Stream.Finish {
		t.Fatalf("expected finish true")
	}
}

func TestVerifyURLHandlesDecodedQueryValue(t *testing.T) {
	token := "token"
	rawKey := bytes.Repeat([]byte{0x34}, 32)
	encodingAESKey := strings.TrimRight(base64.StdEncoding.EncodeToString(rawKey), "=")
	corpID := "corp-id"
	crypt, err := NewCrypt(token, encodingAESKey, corpID)
	if err != nil {
		t.Fatalf("create crypt: %v", err)
	}

	var (
		echostr      string
		expectedBody string
	)
	for i := 0; i < 512; i++ {
		extra := fmt.Sprintf("payload-%d", i)
		enc, err := crypt.encrypt([]byte(extra))
		if err != nil {
			t.Fatalf("encrypt: %v", err)
		}
		if strings.Contains(enc, "+") {
			echostr = enc
			expectedBody = extra
			break
		}
	}
	if echostr == "" {
		t.Skip("unable to generate test data containing '+'; try rerun")
	}

	timestamp := "1761891968"
	nonce := "random-nonce"
	signature := calcSignature(token, timestamp, nonce, echostr)

	values := url.Values{}
	values.Set("msg_signature", signature)
	values.Set("timestamp", timestamp)
	values.Set("nonce", nonce)
	values.Set("echostr", echostr)
	rawQuery := values.Encode() // 会对 '+' 进行 %2B 转义
	req, err := http.NewRequest(http.MethodGet, "/callback?"+rawQuery, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	decoded := req.URL.Query().Get("echostr")
	if decoded != echostr {
		t.Fatalf("unexpected decoded value: %q", decoded)
	}

	plain, err := crypt.VerifyURL(signature, timestamp, nonce, decoded)
	if err != nil {
		t.Fatalf("verify url: %v", err)
	}
	if plain != expectedBody {
		t.Fatalf("unexpected plaintext: %q", plain)
	}
}

func TestVerifyURLRoundTrip(t *testing.T) {
	token := "sample-token"
	rawKey := bytes.Repeat([]byte{0x44}, 32)
	encodingAESKey := strings.TrimRight(base64.StdEncoding.EncodeToString(rawKey), "=")
	corpID := "sample-corp-id"
	crypt, err := NewCrypt(token, encodingAESKey, corpID)
	if err != nil {
		t.Fatalf("create crypt: %v", err)
	}

	payload := []byte("roundtrip-payload")
	echoStr, err := crypt.encrypt(payload)
	if err != nil {
		t.Fatalf("encrypt sample payload: %v", err)
	}

	timestamp := "1761891968"
	nonce := "nonce"
	signature := calcSignature(token, timestamp, nonce, echoStr)

	plain, err := crypt.VerifyURL(signature, timestamp, nonce, echoStr)
	if err != nil {
		t.Fatalf("verify url: %v", err)
	}

	if plain != string(payload) {
		t.Fatalf("unexpected plaintext: %s", plain)
	}
}

func TestDecryptMessageWithDocSample(t *testing.T) {
	token := "QDG6eK"
	encodingAESKey := "jWmYm7qr5nMoAUwZRjGtBxmz3KA1tkAj3ykkR6q2B2C"
	corpID := "wx5823bf96d3bd56c7"
	crypt, err := NewCrypt(token, encodingAESKey, corpID)
	if err != nil {
		t.Fatalf("create crypt: %v", err)
	}

	const cipherText = "RypEvHKD8QQKFhvQ6QleEB4J58tiPdvo+rtK1I9qca6aM/wvqnLSV5zEPeusUiX5L5X/0lWfrf0QADHHhGd3QczcdCUpj911L3vg3W/sYYvuJTs3TUUkSUXxaccAS0qhxchrRYt66wiSpGLYL42aM6A8dTT+6k4aSknmPj48kzJs8qLjvd4Xgpue06DOdnLxAUHzM6+kDZ+HMZfJYuR+LtwGc2hgf5gsijff0ekUNXZiqATP7PF5mZxZ3Izoun1s4zG4LUMnvw2r+KqCKIw+3IQH03v+BCA9nMELNqbSf6tiWSrXJB3LAVGUcallcrw8V2t9EL4EhzJWrQUax5wLVMNS0+rUPA3k22Ncx4XXZS9o0MBH27Bo6BpNelZpS+/uh9KsNlY6bHCmJU9p8g7m3fVKn28H3KDYA5Pl/T8Z1ptDAVe0lXdQ2YoyyH2uyPIGHBZZIs2pDBS8R07+qN+E7Q=="
	plain, err := crypt.decrypt(cipherText)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	const expectedXML = `<xml><ToUserName><![CDATA[wx5823bf96d3bd56c7]]></ToUserName>
<FromUserName><![CDATA[mycreate]]></FromUserName>
<CreateTime>1409659813</CreateTime>
<MsgType><![CDATA[text]]></MsgType>
<Content><![CDATA[hello]]></Content>
<MsgId>4561255354251345929</MsgId>
<AgentID>218</AgentID>
</xml>`
	if string(plain) != expectedXML {
		t.Fatalf("unexpected plaintext:\n%s", plain)
	}
}
