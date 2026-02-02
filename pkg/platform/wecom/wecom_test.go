// Package wecom tests cover crypt/stream/bot 的协议兼容与关键行为。
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

// TestCalcSignatureDeterministic 验证签名算法具备确定性。
func TestCalcSignatureDeterministic(t *testing.T) {
	sig1 := calcSignature("token", "12345", "nonce", "cipher")
	sig2 := calcSignature("token", "12345", "nonce", "cipher")
	if sig1 != sig2 {
		t.Fatalf("signature mismatch: %s vs %s", sig1, sig2)
	}
}

// TestCryptEncryptDecryptRoundTrip 验证加解密流程能完整往返。
func TestCryptEncryptDecryptRoundTrip(t *testing.T) {
	rawKey := bytes.Repeat([]byte{0x11}, 32)
	encodingKey := base64.StdEncoding.EncodeToString(rawKey)
	// 企业微信 EncodingAESKey 为 43 字节 Base64 字符串，需去掉末尾 '='。
	encodingKey = strings.TrimRight(encodingKey, "=")
	crypt, err := NewCrypt("token", encodingKey, "corpID")
	if err != nil {
		t.Fatalf("create crypt: %v", err)
	}
	payload := buildStreamReply("stream-id", "hello", false)
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

// TestSessionManagerPublishConsume 验证流式片段累积与过期清理逻辑。
func TestSessionManagerPublishConsume(t *testing.T) {
	mgr := newStreamManager(50*time.Millisecond, 10*time.Millisecond)
	msg := &Message{MsgID: "mid", ChatID: "cid", From: MessageSender{UserID: "uid"}}
	session, isNew := mgr.createOrGet(msg)
	if !isNew {
		t.Fatalf("expected new session")
	}
	if session.StreamID == "" {
		t.Fatalf("empty stream id")
	}
	if _, ok := mgr.getStreamIDByMsg("mid"); !ok {
		t.Fatalf("msg id not indexed")
	}

	// publish two chunks rapidly
	mgr.publish(session.StreamID, botcore.StreamChunk{Content: "chunk1"})
	mgr.publish(session.StreamID, botcore.StreamChunk{Content: "final", IsFinal: true})

	// consume with a small timeout - should drain both and return merged result
	_, chunk := mgr.getLatestChunk(session.StreamID)
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
	mgr.cleanup()
	if _, ok := mgr.getStreamIDByMsg("mid"); ok {
		t.Fatalf("session not cleaned")
	}
}

// TestBotRefreshFinalMessage 验证 setFinalMessage 会被 refresh 拉取返回。
func TestBotRefreshFinalMessage(t *testing.T) {
	rawKey := bytes.Repeat([]byte{0x22}, 32)
	key := strings.TrimRight(base64.StdEncoding.EncodeToString(rawKey), "=")
	crypt, err := NewCrypt("token", key, "corpID")
	if err != nil {
		t.Fatalf("create crypt: %v", err)
	}
	mgr := newStreamManager(time.Minute, 5*time.Millisecond)
	bot := &Bot{streamMgr: mgr, crypto: crypt}
	msg := &Message{MsgID: "mid", MsgType: "text", Text: &TextPayload{Content: "hi"}, From: MessageSender{UserID: "uid"}}
	ts := "1700000000"
	initialResp, err := bot.initial(msg, ts, "nonce")
	if err != nil {
		t.Fatalf("initial: %v", err)
	}
	if initialResp.Encrypt == "" {
		t.Fatalf("empty encrypt in initial resp")
	}
	sessionID, ok := mgr.getStreamIDByMsg("mid")
	if !ok {
		t.Fatalf("missing stream id")
	}
	bot.setFinalMessage("mid", "done")
	refresh := &Message{MsgID: "mid", MsgType: "stream", Stream: &StreamPayload{ID: sessionID}}
	refreshResp, err := bot.refresh(refresh, ts, "nonce")
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

// TestBotInitialReturnsEmptyAckAndRefreshGetsPipelineOutput 验证首包空 ACK + refresh 拉取流水线输出。
func TestBotInitialReturnsEmptyAckAndRefreshGetsPipelineOutput(t *testing.T) {
	rawKey := bytes.Repeat([]byte{0x33}, 32)
	key := strings.TrimRight(base64.StdEncoding.EncodeToString(rawKey), "=")
	crypt, err := NewCrypt("token", key, "corpID")
	if err != nil {
		t.Fatalf("create crypt: %v", err)
	}
	pipeline := botcore.PipelineFunc(func(update botcore.RequestSnapshot) <-chan botcore.StreamChunk {
		ch := make(chan botcore.StreamChunk, 2)
		ch <- botcore.StreamChunk{Content: "hi ", IsFinal: false}
		ch <- botcore.StreamChunk{Content: "there", IsFinal: true}
		close(ch)
		return ch
	})
	mgr := newStreamManager(time.Minute, 200*time.Millisecond)
	bot := &Bot{streamMgr: mgr, crypto: crypt, pipeline: pipeline}
	msg := &Message{MsgID: "mid", MsgType: "text", Text: &TextPayload{Content: "hello"}, From: MessageSender{UserID: "uid"}}
	ts := "1700000000"

	initialResp, err := bot.initial(msg, ts, "nonce")
	if err != nil {
		t.Fatalf("initial: %v", err)
	}
	initialPlain, err := crypt.DecryptMessage(initialResp.MsgSignature, initialResp.Timestamp, initialResp.Nonce, EncryptedRequest{Encrypt: initialResp.Encrypt})
	if err != nil {
		t.Fatalf("decrypt initial: %v", err)
	}
	if initialPlain.Stream == nil {
		t.Fatalf("missing stream payload in initial")
	}
	if initialPlain.Stream.Content != "" {
		t.Fatalf("expected empty initial content, got %q", initialPlain.Stream.Content)
	}
	if initialPlain.Stream.Finish {
		t.Fatalf("expected initial finish false")
	}
	sessionID, ok := mgr.getStreamIDByMsg("mid")
	if !ok || sessionID == "" {
		t.Fatalf("missing stream id")
	}
	if initialPlain.Stream.ID != sessionID {
		t.Fatalf("unexpected stream id: %s", initialPlain.Stream.ID)
	}

	refresh := &Message{MsgID: "mid", MsgType: "stream", Stream: &StreamPayload{ID: sessionID}}
	refreshResp, err := bot.refresh(refresh, ts, "nonce")
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	refreshPlain, err := crypt.DecryptMessage(refreshResp.MsgSignature, refreshResp.Timestamp, refreshResp.Nonce, EncryptedRequest{Encrypt: refreshResp.Encrypt})
	if err != nil {
		t.Fatalf("decrypt refresh: %v", err)
	}
	if refreshPlain.Stream == nil {
		t.Fatalf("missing stream payload in refresh")
	}
	if !refreshPlain.Stream.Finish {
		// 关键步骤：首轮 refresh 可能只取到部分内容，继续拉取直到完成。
		refreshResp, err = bot.refresh(refresh, ts, "nonce")
		if err != nil {
			t.Fatalf("refresh second: %v", err)
		}
		refreshPlain, err = crypt.DecryptMessage(refreshResp.MsgSignature, refreshResp.Timestamp, refreshResp.Nonce, EncryptedRequest{Encrypt: refreshResp.Encrypt})
		if err != nil {
			t.Fatalf("decrypt refresh second: %v", err)
		}
		if refreshPlain.Stream == nil {
			t.Fatalf("missing stream payload in refresh second")
		}
	}
	if refreshPlain.Stream.Content != "hi there" {
		t.Fatalf("unexpected content: %s", refreshPlain.Stream.Content)
	}
	if !refreshPlain.Stream.Finish {
		t.Fatalf("expected finish true")
	}
}

// TestBotInitialNoResponseEndsStream 验证 NoResponse 会直接结束流式会话。
func TestBotInitialNoResponseEndsStream(t *testing.T) {
	rawKey := bytes.Repeat([]byte{0x66}, 32)
	key := strings.TrimRight(base64.StdEncoding.EncodeToString(rawKey), "=")
	crypt, err := NewCrypt("token", key, "corpID")
	if err != nil {
		t.Fatalf("create crypt: %v", err)
	}
	pipeline := botcore.PipelineFunc(func(update botcore.RequestSnapshot) <-chan botcore.StreamChunk {
		ch := make(chan botcore.StreamChunk, 1)
		ch <- botcore.StreamChunk{Payload: botcore.NoResponse}
		close(ch)
		return ch
	})
	mgr := newStreamManager(time.Minute, 200*time.Millisecond)
	bot := &Bot{streamMgr: mgr, crypto: crypt, pipeline: pipeline}
	msg := &Message{MsgID: "mid", MsgType: "text", Text: &TextPayload{Content: "hello"}, From: MessageSender{UserID: "uid"}}
	ts := "1700000000"

	initialResp, err := bot.initial(msg, ts, "nonce")
	if err != nil {
		t.Fatalf("initial: %v", err)
	}
	initialPlain, err := crypt.DecryptMessage(initialResp.MsgSignature, initialResp.Timestamp, initialResp.Nonce, EncryptedRequest{Encrypt: initialResp.Encrypt})
	if err != nil {
		t.Fatalf("decrypt initial: %v", err)
	}
	if initialPlain.Stream == nil {
		t.Fatalf("missing stream payload in initial")
	}
	if initialPlain.Stream.Content != "" {
		t.Fatalf("expected empty initial content, got %q", initialPlain.Stream.Content)
	}
	if initialPlain.Stream.Finish {
		t.Fatalf("expected initial finish false")
	}
	sessionID, ok := mgr.getStreamIDByMsg("mid")
	if !ok || sessionID == "" {
		t.Fatalf("missing stream id")
	}

	refresh := &Message{MsgID: "mid", MsgType: "stream", Stream: &StreamPayload{ID: sessionID}}
	refreshResp, err := bot.refresh(refresh, ts, "nonce")
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	refreshPlain, err := crypt.DecryptMessage(refreshResp.MsgSignature, refreshResp.Timestamp, refreshResp.Nonce, EncryptedRequest{Encrypt: refreshResp.Encrypt})
	if err != nil {
		t.Fatalf("decrypt refresh: %v", err)
	}
	if refreshPlain.Stream == nil {
		t.Fatalf("missing stream payload in refresh")
	}
	if refreshPlain.Stream.Content != "" {
		t.Fatalf("unexpected content: %s", refreshPlain.Stream.Content)
	}
	if !refreshPlain.Stream.Finish {
		t.Fatalf("expected finish true")
	}
}

// TestVerifyURLHandlesDecodedQueryValue 验证 URL 解码导致的 '+' 还原场景。
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

// TestVerifyURLRoundTrip 验证 URL 验证流程的加解密往返。
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

// TestDecryptMessageWithDocSample 使用官方样例密文验证解密兼容性。
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
