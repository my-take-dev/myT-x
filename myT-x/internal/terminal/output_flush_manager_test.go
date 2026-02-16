package terminal

import (
	"sync"
	"testing"
	"time"
)

func TestOutputFlushManagerFlushesOnThreshold(t *testing.T) {
	ch := make(chan string, 2)
	manager := NewOutputFlushManager(time.Hour, 5, func(paneID string, data []byte) {
		ch <- paneID + ":" + string(data)
	})
	manager.Start()
	defer manager.Stop()

	manager.Write("%1", []byte("abc"))
	manager.Write("%1", []byte("de"))

	select {
	case got := <-ch:
		if got != "%1:abcde" {
			t.Fatalf("flush payload = %q, want %q", got, "%1:abcde")
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("expected threshold flush")
	}
}

func TestOutputFlushManagerTickerFlushes(t *testing.T) {
	ch := make(chan string, 2)
	manager := NewOutputFlushManager(15*time.Millisecond, 1024, func(paneID string, data []byte) {
		ch <- paneID + ":" + string(data)
	})
	manager.Start()
	defer manager.Stop()

	manager.Write("%2", []byte("tick"))

	select {
	case got := <-ch:
		if got != "%2:tick" {
			t.Fatalf("flush payload = %q, want %q", got, "%2:tick")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected periodic flush")
	}
}

func TestOutputFlushManagerRetainPanesFlushesRemoved(t *testing.T) {
	var (
		mu      sync.Mutex
		emitted []string
	)
	manager := NewOutputFlushManager(time.Hour, 1024, func(paneID string, data []byte) {
		mu.Lock()
		emitted = append(emitted, paneID+":"+string(data))
		mu.Unlock()
	})
	manager.Start()
	defer manager.Stop()

	manager.Write("%1", []byte("keep"))
	manager.Write("%2", []byte("drop"))
	removed := manager.RetainPanes(map[string]struct{}{"%1": {}})
	if len(removed) != 1 || removed[0] != "%2" {
		t.Fatalf("removed = %#v, want [%q]", removed, "%2")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(emitted) != 1 || emitted[0] != "%2:drop" {
		t.Fatalf("emitted = %#v, want [%q]", emitted, "%2:drop")
	}
}

func TestOutputFlushManagerStopFlushesPending(t *testing.T) {
	ch := make(chan string, 2)
	manager := NewOutputFlushManager(time.Hour, 1024, func(paneID string, data []byte) {
		ch <- paneID + ":" + string(data)
	})
	manager.Start()
	manager.Write("%9", []byte("pending"))
	manager.Stop()

	select {
	case got := <-ch:
		if got != "%9:pending" {
			t.Fatalf("flush payload = %q, want %q", got, "%9:pending")
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("expected flush on stop")
	}
}

func TestOutputFlushManagerRemovePaneFlushesPending(t *testing.T) {
	ch := make(chan string, 4)
	manager := NewOutputFlushManager(time.Hour, 1024, func(paneID string, data []byte) {
		ch <- paneID + ":" + string(data)
	})
	manager.Start()
	defer manager.Stop()

	manager.Write("%1", []byte("left"))
	manager.Write("%2", []byte("right"))
	manager.RemovePane("%1")

	select {
	case got := <-ch:
		if got != "%1:left" {
			t.Fatalf("flush payload = %q, want %q", got, "%1:left")
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("expected flush on RemovePane")
	}

	removed := manager.RetainPanes(map[string]struct{}{"%2": {}})
	if len(removed) != 0 {
		t.Fatalf("removed = %#v, want empty", removed)
	}
}

func TestOutputFlushManagerWriteEdgeCasesAreNoop(t *testing.T) {
	ch := make(chan string, 2)
	manager := NewOutputFlushManager(15*time.Millisecond, 1, func(paneID string, data []byte) {
		ch <- paneID + ":" + string(data)
	})
	manager.Start()
	defer manager.Stop()

	manager.Write("", []byte("x"))
	manager.Write("%1", nil)
	manager.Write("%1", []byte{})

	select {
	case got := <-ch:
		t.Fatalf("unexpected flush for noop writes: %q", got)
	case <-time.After(120 * time.Millisecond):
	}
}

func TestOutputFlushManagerWriteAfterStopIsIgnored(t *testing.T) {
	ch := make(chan string, 2)
	manager := NewOutputFlushManager(15*time.Millisecond, 1, func(paneID string, data []byte) {
		ch <- paneID + ":" + string(data)
	})
	manager.Start()
	manager.Stop()

	manager.Write("%1", []byte("ignored"))

	select {
	case got := <-ch:
		t.Fatalf("unexpected flush after stop: %q", got)
	case <-time.After(120 * time.Millisecond):
	}
}

func TestOutputFlushManagerStopBeforeStartIsIdempotent(t *testing.T) {
	manager := NewOutputFlushManager(15*time.Millisecond, 1, func(string, []byte) {})
	manager.Stop()
	manager.Stop()
}
