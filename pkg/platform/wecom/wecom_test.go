// Package wecom tests cover Bot SDK integration.
package wecom

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"

	wecomproto "github.com/IMBotPlatform/bot-protocol-wecom/pkg/wecom"
)

// TestCalcSignatureDeterministic 验证签名算法具备确定性。
func TestCalcSignatureDeterministic(t *testing.T) {
	sig1 := CalcSignature("token", "12345", "nonce", "cipher")
	sig2 := CalcSignature("token", "12345", "nonce", "cipher")
	if sig1 != sig2 {
		t.Fatalf("signature mismatch: %s vs %s", sig1, sig2)
	}
}

// TestCryptEncryptDecryptRoundTrip 验证加解密流程能完整往返。
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
	msg, err := crypt.DecryptMessage(resp.MsgSignature, resp.Timestamp, resp.Nonce, wecomproto.EncryptedRequest{Encrypt: resp.Encrypt})
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

// TestNewBotCreation 验证 Bot 创建成功。
func TestNewBotCreation(t *testing.T) {
	rawKey := bytes.Repeat([]byte{0x22}, 32)
	key := strings.TrimRight(base64.StdEncoding.EncodeToString(rawKey), "=")
	bot, err := NewBot("token", key, "corpID", 0, 0, nil)
	if err != nil {
		t.Fatalf("create bot: %v", err)
	}
	if bot == nil {
		t.Fatalf("bot is nil")
	}
}

// TestPipelineAdapterNilPipeline 验证空 pipeline 不会 panic。
func TestPipelineAdapterNilPipeline(t *testing.T) {
	adapter := NewPipelineAdapter(nil)
	ch := adapter.Handle(wecomproto.Context{})
	if ch != nil {
		t.Fatalf("expected nil channel for nil pipeline")
	}
}
