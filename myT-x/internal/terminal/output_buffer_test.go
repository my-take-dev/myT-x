package terminal

import (
	"testing"
	"time"
)

func TestOutputBufferFlushesOnSizeThreshold(t *testing.T) {
	ch := make(chan []byte, 2)
	buffer := NewOutputBuffer(time.Hour, 5, func(s []byte) {
		ch <- s
	})
	buffer.Start()

	buffer.Write([]byte("abc"))
	buffer.Write([]byte("de"))

	select {
	case got := <-ch:
		if string(got) != "abcde" {
			t.Fatalf("flush = %q, want %q", string(got), "abcde")
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("expected flush on size threshold")
	}

	buffer.Stop()
}

func TestOutputBufferStopFlushesPendingData(t *testing.T) {
	ch := make(chan []byte, 2)
	buffer := NewOutputBuffer(time.Hour, 1024, func(s []byte) {
		ch <- s
	})
	buffer.Start()
	buffer.Write([]byte("pending"))
	buffer.Stop()

	select {
	case got := <-ch:
		if string(got) != "pending" {
			t.Fatalf("flush = %q, want %q", string(got), "pending")
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("expected flush on Stop")
	}
}

func TestOutputBufferTickerFlushesData(t *testing.T) {
	ch := make(chan []byte, 2)
	buffer := NewOutputBuffer(15*time.Millisecond, 1024, func(s []byte) {
		ch <- s
	})
	buffer.Start()
	buffer.Write([]byte("tick"))

	select {
	case got := <-ch:
		if string(got) != "tick" {
			t.Fatalf("flush = %q, want %q", string(got), "tick")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected flush on ticker")
	}

	buffer.Stop()
}

func TestOutputBufferFlushesWhileWritesAreContinuous(t *testing.T) {
	ch := make(chan []byte, 4)
	buffer := NewOutputBuffer(20*time.Millisecond, 1024, func(s []byte) {
		ch <- s
	})
	buffer.Start()
	defer buffer.Stop()

	deadline := time.Now().Add(130 * time.Millisecond)
	for time.Now().Before(deadline) {
		buffer.Write([]byte("x"))
		time.Sleep(5 * time.Millisecond)
	}

	select {
	case got := <-ch:
		if len(got) == 0 {
			t.Fatal("flush payload should not be empty")
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("expected bounded flush under continuous writes")
	}
}
