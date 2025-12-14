package command

import (
	"github.com/IMBotPlatform/IMBotCore/botcore"
)

// StreamWriter 实现 io.Writer 接口，将输出重定向到 StreamChunk 通道。
// 这允许 Cobra 命令像操作 stdout 一样直接打印，而结果会被流式传输给用户。
type StreamWriter struct {
	Ch chan<- botcore.StreamChunk
}

// NewStreamWriter 创建一个新的 StreamWriter。
func NewStreamWriter(ch chan<- botcore.StreamChunk) *StreamWriter {
	return &StreamWriter{Ch: ch}
}

// Write 将字节切片转换为 StreamChunk 发送。
func (w *StreamWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	// 注意：这里直接将字节转为字符串。
	// 如果 Cobra 输出非常碎小的包，可能需要在此处做缓冲（Buffer）。
	// 但对于常规 CLI 输出，直接转发通常是可以接受的，也能体现“流式”感。
	msg := string(p)
	w.Ch <- botcore.StreamChunk{
		Content: msg,
		IsFinal: false,
	}
	return len(p), nil
}
