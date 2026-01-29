package websocket

import (
	"testing"
	"time"
)

func TestDataProcessorStream_EmitsAfterChunk(t *testing.T) {
	dp := NewDataProcessorStream(
		4,  // chunkBytes
		32, // maxBytes
		4,  // trimBytes
	)

	dp.Start(10 * time.Millisecond)
	defer dp.Stop()

	dp.Push([]byte("abcd")) // exactly one chunk

	select {
	case ev := <-dp.Output():
		if string(ev.body) != "abcd" {
			t.Fatalf("expected 'abcd', got '%s'", string(ev.body))
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}

func TestDataProcessorStream_DoesNotEmitBeforeChunk(t *testing.T) {
	dp := NewDataProcessorStream(4, 32, 4)
	dp.Start(10 * time.Millisecond)
	defer dp.Stop()

	dp.Push([]byte("ab")) // not enough for chunk

	select {
	case ev := <-dp.Output():
		t.Fatalf("unexpected event: %v", ev)
	case <-time.After(50 * time.Millisecond):
		// success
	}
}

func TestDataProcessorStream_RollingBuffer(t *testing.T) {
	dp := NewDataProcessorStream(
		4, // chunkBytes
		8, // maxBytes
		4, // trimBytes
	)

	dp.Start(10 * time.Millisecond)
	defer dp.Stop()

	dp.Push([]byte("abcd")) // buffer: abcd
	dp.Push([]byte("efgh")) // buffer: abcdefgh

	<-dp.Output() // first emission

	dp.Push([]byte("ijkl")) // buffer should trim to efghijkl

	select {
	case ev := <-dp.Output():
		if string(ev.body) != "efghijkl" {
			t.Fatalf("expected rolling buffer 'efghijkl', got '%s'", string(ev.body))
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}

func TestDataProcessorStream_MultipleChunksFromSinglePush(t *testing.T) {
	dp := NewDataProcessorStream(4, 32, 4)
	dp.Start(10 * time.Millisecond)
	defer dp.Stop()

	dp.Push([]byte("abcdefgh")) // 2 chunks

	select {
	case ev := <-dp.Output():
		if string(ev.body) != "abcdefgh" {
			t.Fatalf("expected 'abcdefgh', got '%s'", string(ev.body))
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}
