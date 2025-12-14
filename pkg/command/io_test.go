package command

import (
	"testing"

	"github.com/IMBotPlatform/IMBotCore/pkg/botcore"
)

func TestStreamWriterIncremental(t *testing.T) {
	// Create a buffered channel to hold chunks
	ch := make(chan botcore.StreamChunk, 10)
	w := NewStreamWriter(ch)

	// Simulate writing "Hello"
	_, err := w.Write([]byte("Hello"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Simulate writing " World"
	_, err = w.Write([]byte(" World"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Consume the first chunk
	select {
	case chunk1 := <-ch:
		if chunk1.Content != "Hello" {
			t.Errorf("Expected first chunk 'Hello', got '%s'", chunk1.Content)
		}
	default:
		t.Fatal("Expected chunk available")
	}

	// Consume the second chunk - it should contain ONLY the incremental content
	select {
	case chunk2 := <-ch:
		if chunk2.Content != " World" {
			t.Errorf("Expected second chunk ' World' (incremental), got '%s'", chunk2.Content)
		}
	default:
		t.Fatal("Expected second chunk available")
	}
}
