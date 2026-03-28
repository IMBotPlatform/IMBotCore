// Package wecom tests cover Bot SDK integration.
package wecom

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
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

func TestBuildSnapshotIncludesReferenceAndSharedKeyTransform(t *testing.T) {
	rawKey := bytes.Repeat([]byte{0x33}, 32)
	key := strings.TrimRight(base64.StdEncoding.EncodeToString(rawKey), "=")
	bot, err := wecomproto.NewBot("token", key, "corpID", nil)
	if err != nil {
		t.Fatalf("create bot: %v", err)
	}

	msg := &wecomproto.Message{
		MsgType: "file",
		From:    wecomproto.MessageSender{UserID: "user-1"},
		File:    &wecomproto.FilePayload{URL: "https://example.com/file.bin"},
		Quote: &wecomproto.QuotePayload{
			MsgType: "text",
			Text:    &wecomproto.TextPayload{Content: "quoted text"},
		},
	}

	snapshot := buildSnapshot(wecomproto.Context{
		Message:  msg,
		StreamID: "stream-1",
		Bot:      bot,
	})

	if snapshot.Reference == nil {
		t.Fatal("reference should not be nil")
	}
	if snapshot.Reference.Text != "quoted text" {
		t.Fatalf("unexpected reference text: %s", snapshot.Reference.Text)
	}
	if len(snapshot.Attachments) != 1 {
		t.Fatalf("unexpected attachments length: %d", len(snapshot.Attachments))
	}
	if snapshot.Attachments[0].DownloadTransform == nil {
		t.Fatal("file attachment should have download transform")
	}

	plain := []byte("shared-key-file")
	cipherData, err := encryptDownloadedFileForTest(rawKey, plain)
	if err != nil {
		t.Fatalf("encrypt file payload: %v", err)
	}
	got, err := snapshot.Attachments[0].DownloadTransform(cipherData)
	if err != nil {
		t.Fatalf("transform attachment: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("unexpected transform result: got=%q want=%q", string(got), string(plain))
	}
}

func TestBuildSnapshotUsesResourceAESKeyTransform(t *testing.T) {
	rawKey := bytes.Repeat([]byte{0x44}, 32)
	resourceAESKey := strings.TrimRight(base64.StdEncoding.EncodeToString(rawKey), "=")

	msg := &wecomproto.Message{
		MsgType: "video",
		Video: &wecomproto.VideoPayload{
			URL:    "https://example.com/video.bin",
			AESKey: resourceAESKey,
		},
		Quote: &wecomproto.QuotePayload{
			MsgType: "file",
			File: &wecomproto.FilePayload{
				URL:    "https://example.com/ref-file.bin",
				AESKey: resourceAESKey,
			},
		},
	}

	snapshot := buildSnapshot(wecomproto.Context{Message: msg, StreamID: "stream-2"})
	if len(snapshot.Attachments) != 1 {
		t.Fatalf("unexpected attachments length: %d", len(snapshot.Attachments))
	}
	if snapshot.Reference == nil || len(snapshot.Reference.Attachments) != 1 {
		t.Fatalf("unexpected reference attachments: %+v", snapshot.Reference)
	}

	plain := []byte("resource-key-file")
	cipherData, err := encryptDownloadedFileForTest(rawKey, plain)
	if err != nil {
		t.Fatalf("encrypt file payload: %v", err)
	}

	got, err := snapshot.Reference.Attachments[0].DownloadTransform(cipherData)
	if err != nil {
		t.Fatalf("transform attachment: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("unexpected transform result: got=%q want=%q", string(got), string(plain))
	}
}

func encryptDownloadedFileForTest(aesKey, plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}
	iv := aesKey[:aes.BlockSize]
	buf := pkcs7PadForTest(plain, 32)
	mode := cipher.NewCBCEncrypter(block, iv)
	cipherData := make([]byte, len(buf))
	mode.CryptBlocks(cipherData, buf)
	return cipherData, nil
}

func pkcs7PadForTest(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	out := make([]byte, len(data)+padding)
	copy(out, data)
	for i := len(data); i < len(out); i++ {
		out[i] = byte(padding)
	}
	return out
}
