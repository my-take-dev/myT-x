package main

import (
	"bytes"
	"context"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"myT-x/internal/panestate"
)

func waitForCondition(t *testing.T, timeout time.Duration, condition func() bool, message string) {
	t.Helper()
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()

	for {
		if condition() {
			return
		}
		select {
		case <-deadline.C:
			t.Fatalf("timeout waiting for condition: %s", message)
		case <-ticker.C:
		}
	}
}

func TestStartPaneFeedWorkerLifecycle(t *testing.T) {
	t.Run("starts and processes items", func(t *testing.T) {
		app := NewApp()
		app.paneStates = panestate.NewManager(4096)
		app.paneStates.EnsurePane("%0", 80, 24)
		app.paneStates.SetActivePanes(map[string]struct{}{"%0": {}})

		ctx := context.Background()
		app.startPaneFeedWorker(ctx)
		defer app.stopPaneFeedWorker()

		// Enqueue data through the channel.
		chunk := []byte("hello worker")
		pooled, bp := getFeedBuffer(len(chunk))
		copy(pooled, chunk)
		*bp = pooled
		app.paneFeedCh <- paneFeedItem{
			paneID:  "%0",
			chunk:   pooled,
			poolPtr: bp,
		}

		waitForCondition(t, 2*time.Second, func() bool {
			snap := app.paneStates.Snapshot("%0")
			return strings.Contains(snap, "hello worker")
		}, "pane feed worker should process queued chunk")

		snap := app.paneStates.Snapshot("%0")
		if !strings.Contains(snap, "hello worker") {
			t.Errorf("worker should have processed feed item, snapshot = %q", snap)
		}
	})

	t.Run("context cancellation stops worker", func(t *testing.T) {
		app := NewApp()
		app.paneStates = panestate.NewManager(4096)
		app.paneStates.EnsurePane("%0", 80, 24)

		ctx, cancel := context.WithCancel(context.Background())
		app.startPaneFeedWorker(ctx)

		cancel()
		app.bgWG.Wait() // Worker goroutine should exit.
	})

	t.Run("nil paneStates is no-op", func(t *testing.T) {
		app := NewApp()
		app.paneStates = nil
		// Should not panic.
		app.startPaneFeedWorker(context.Background())
		app.stopPaneFeedWorker()
	})
}

func TestEnqueuePaneStateFeed(t *testing.T) {
	t.Run("normal enqueue via channel", func(t *testing.T) {
		app := NewApp()
		app.paneStates = panestate.NewManager(4096)
		app.paneStates.EnsurePane("%0", 80, 24)
		app.paneStates.SetActivePanes(map[string]struct{}{"%0": {}})

		ctx := context.Background()
		app.startPaneFeedWorker(ctx)
		defer app.stopPaneFeedWorker()

		app.enqueuePaneStateFeed("%0", []byte("test data"))

		waitForCondition(t, 2*time.Second, func() bool {
			snap := app.paneStates.Snapshot("%0")
			return strings.Contains(snap, "test data")
		}, "pane feed enqueue should be consumed by worker")

		snap := app.paneStates.Snapshot("%0")
		if !strings.Contains(snap, "test data") {
			t.Errorf("data should be processed via channel, snapshot = %q", snap)
		}
	})

	t.Run("empty chunk is ignored", func(t *testing.T) {
		app := NewApp()
		app.paneStates = panestate.NewManager(4096)
		app.paneStates.EnsurePane("%0", 80, 24)

		initialLen := len(app.paneFeedCh)
		app.enqueuePaneStateFeed("%0", []byte{})
		app.enqueuePaneStateFeed("%0", nil)

		if len(app.paneFeedCh) != initialLen {
			t.Error("empty chunks should not be enqueued")
		}
	})

	t.Run("nil paneStates is no-op", func(t *testing.T) {
		app := NewApp()
		app.paneStates = nil
		// Should not panic.
		app.enqueuePaneStateFeed("%0", []byte("data"))
	})

	t.Run("empty paneID is enqueued without panic", func(t *testing.T) {
		app := NewApp()
		app.paneStates = panestate.NewManager(4096)

		ctx := context.Background()
		app.startPaneFeedWorker(ctx)
		defer app.stopPaneFeedWorker()

		// Should not panic even with empty paneID.
		app.enqueuePaneStateFeed("", []byte("data for empty paneID"))

		// Give worker time to consume the item.
		time.Sleep(50 * time.Millisecond)

		// Verify the channel was consumed (item was processed, even if paneID
		// doesn't correspond to a real pane).
		if got := len(app.paneFeedCh); got != 0 {
			t.Fatalf("paneFeedCh length = %d, want 0 after worker consumes item", got)
		}
	})

	t.Run("channel full falls back to direct Feed", func(t *testing.T) {
		var logs bytes.Buffer
		originalLogger := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
		t.Cleanup(func() {
			slog.SetDefault(originalLogger)
		})

		app := NewApp()
		app.paneFeedCh = make(chan paneFeedItem, 1) // Tiny buffer to force fallback.
		app.paneStates.EnsurePane("%0", 80, 24)
		app.paneStates.SetActivePanes(map[string]struct{}{"%0": {}})

		// Fill the channel.
		dummy, bp := getFeedBuffer(1)
		dummy[0] = 'X'
		*bp = dummy
		app.paneFeedCh <- paneFeedItem{paneID: "%0", chunk: dummy, poolPtr: bp}

		// This enqueue should fall back to direct Feed.
		app.enqueuePaneStateFeed("%0", []byte("fallback data"))

		snap := app.paneStates.Snapshot("%0")
		if !strings.Contains(snap, "fallback data") {
			t.Errorf("fallback direct Feed should have written data, snapshot = %q", snap)
		}
		// Check for log message (format varies based on handler, so check for key content)
		logOutput := logs.String()
		if !strings.Contains(logOutput, "channel full") || !strings.Contains(logOutput, "direct feed") {
			t.Fatalf("expected channel-full debug log, output=%q", logOutput)
		}
	})
}

func TestStopPaneFeedWorker(t *testing.T) {
	t.Run("double stop is safe", func(t *testing.T) {
		app := NewApp()
		app.paneStates = panestate.NewManager(4096)
		app.startPaneFeedWorker(context.Background())
		app.stopPaneFeedWorker()
		app.stopPaneFeedWorker() // Should not panic.
	})

	t.Run("stop drains queued channel items", func(t *testing.T) {
		app := NewApp()
		app.paneFeedCh = make(chan paneFeedItem, 4)
		chunk, bp := getFeedBuffer(4)
		copy(chunk, []byte("test"))
		app.paneFeedCh <- paneFeedItem{paneID: "%0", chunk: chunk, poolPtr: bp}
		app.stopPaneFeedWorker()
		if got := len(app.paneFeedCh); got != 0 {
			t.Fatalf("paneFeedCh length = %d, want 0 after stop drain", got)
		}
	})

	t.Run("stop nil is safe", func(t *testing.T) {
		app := NewApp()
		app.stopPaneFeedWorker() // Should not panic.
	})
}

func TestFeedBytePoolDataIntegrity(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"small chunk", []byte("hello")},
		{"medium chunk", bytes.Repeat([]byte("X"), 4096)},
		{"large chunk exceeding default cap", bytes.Repeat([]byte("Y"), 16384)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get buffer, copy data, verify, return to pool.
			buf, bp := getFeedBuffer(len(tt.data))
			copy(buf, tt.data)
			if !bytes.Equal(buf, tt.data) {
				t.Fatalf("data mismatch after copy: got %d bytes, want %d", len(buf), len(tt.data))
			}
			*bp = buf
			putFeedBuffer(bp)

			// Get again (may be the same buffer), write different data.
			buf2, bp2 := getFeedBuffer(len(tt.data))
			for i := range buf2 {
				buf2[i] = 'Z'
			}
			expected := bytes.Repeat([]byte("Z"), len(tt.data))
			if !bytes.Equal(buf2, expected) {
				t.Fatalf("reused buffer data mismatch")
			}
			*bp2 = buf2
			putFeedBuffer(bp2)
		})
	}
}

// I-11: putFeedBuffer(nil) must not panic. The nil guard at the top of
// putFeedBuffer is the only protection against a nil pool pointer being
// passed (e.g., from a partially initialised feedItem or a default zero-value).
func TestPutFeedBufferNilDoesNotPanic(t *testing.T) {
	// Must not panic.
	putFeedBuffer(nil)
}

func TestPutFeedBufferDropsBuffersLargerThanMaxPoolSize(t *testing.T) {
	oversized := make([]byte, maxPoolBufSize+1)
	oversizedPtr := &oversized
	putFeedBuffer(oversizedPtr)

	small, smallPtr := getFeedBuffer(1)
	if cap(small) > maxPoolBufSize {
		t.Fatalf("pool returned oversized buffer cap=%d (> %d)", cap(small), maxPoolBufSize)
	}
	*smallPtr = small
	putFeedBuffer(smallPtr)
}

func TestPaneFeedItemFieldCount(t *testing.T) {
	if got := reflect.TypeFor[paneFeedItem]().NumField(); got != 3 {
		t.Fatalf("paneFeedItem field count = %d, want 3; update feed worker/copy logic and tests for new fields", got)
	}
}

func TestFeedBytePoolMaxPoolBufSizeBoundary(t *testing.T) {
	// Verify maxPoolBufSize is 128 KiB and boundary behavior is correct.
	if maxPoolBufSize != 128*1024 {
		t.Fatalf("maxPoolBufSize = %d, want %d (128 KiB)", maxPoolBufSize, 128*1024)
	}

	t.Run("at boundary is returned to pool", func(t *testing.T) {
		// Verify that a buffer with cap == maxPoolBufSize is accepted into the pool.
		// NOTE: sync.Pool makes no guarantee about reuse timing (depends on GC),
		// so we cannot assert that getFeedBuffer returns the same exact buffer.
		// Instead, we verify:
		// 1. putFeedBuffer does not panic or error (accepts the buffer)
		// 2. getFeedBuffer subsequently returns a buffer with capacity <= maxPoolBufSize
		buf := make([]byte, maxPoolBufSize)
		bp := &buf
		putFeedBuffer(bp) // Should not discard: cap == maxPoolBufSize.

		// Get a new buffer and verify it does not exceed the boundary.
		// This indirectly confirms the boundary buffer was accepted into the pool.
		retrieved, rp := getFeedBuffer(1)
		if cap(retrieved) > maxPoolBufSize {
			t.Fatalf("pool returned oversized buffer cap=%d (> %d)", cap(retrieved), maxPoolBufSize)
		}
		*rp = retrieved
		putFeedBuffer(rp)
	})

	t.Run("above boundary is discarded", func(t *testing.T) {
		// Verify that a buffer with cap > maxPoolBufSize is discarded (not pooled).
		// We test this by:
		// 1. Putting an oversized buffer (should be rejected by putFeedBuffer)
		// 2. Getting a small buffer multiple times
		// 3. Confirming each small buffer capacity is still within bounds
		//    (if the oversized buffer had been pooled, a subsequent get might
		//     reuse it and exceed maxPoolBufSize)
		buf := make([]byte, maxPoolBufSize+1)
		bp := &buf
		putFeedBuffer(bp) // Should discard: cap > maxPoolBufSize.

		// Verify pool does not return an oversized buffer.
		// Multiple gets help increase confidence that the oversized buffer
		// was not pooled (though sync.Pool offers no hard guarantee).
		for attempt := range 3 {
			small, smallPtr := getFeedBuffer(1)
			if cap(small) > maxPoolBufSize {
				t.Fatalf("attempt %d: pool returned oversized buffer cap=%d (> %d)",
					attempt, cap(small), maxPoolBufSize)
			}
			*smallPtr = small
			putFeedBuffer(smallPtr)
		}
	})
}

func BenchmarkFeedBytePool(b *testing.B) {
	chunk := bytes.Repeat([]byte("A"), 32*1024) // 32KB typical PTY read

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		buf, bp := getFeedBuffer(len(chunk))
		copy(buf, chunk)
		*bp = buf
		putFeedBuffer(bp)
	}
}
