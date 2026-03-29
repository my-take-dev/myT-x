package snapshot

import (
	"context"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"myT-x/internal/tmux"
	"myT-x/internal/workerutil"
)

// ---------------------------------------------------------------------------
// HandlePaneOutputEvent
// ---------------------------------------------------------------------------

func TestHandlePaneOutputEventAcceptsKnownPayloadTypes(t *testing.T) {
	tests := []struct {
		name    string
		payload any
	}{
		{
			name:    "pointer_to_PaneOutputEvent",
			payload: &tmux.PaneOutputEvent{PaneID: "%0", Data: []byte("hello")},
		},
		{
			name:    "value_PaneOutputEvent",
			payload: tmux.PaneOutputEvent{PaneID: "%0", Data: []byte("world")},
		},
		{
			name: "legacy_map",
			payload: map[string]any{
				"paneId": "%0",
				"data":   []byte("legacy"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestService(t)
			// Should not panic.
			svc.HandlePaneOutputEvent(tt.payload)
		})
	}
}

func TestHandlePaneOutputEventRejectsUnknownPayloads(t *testing.T) {
	svc := newTestService(t)
	// Should not panic; just logs a warning.
	svc.HandlePaneOutputEvent(42)
	svc.HandlePaneOutputEvent("not-a-struct")
	svc.HandlePaneOutputEvent(struct{ X int }{X: 1})
}

func TestHandlePaneOutputEventLegacyMapTypeMismatch(t *testing.T) {
	svc := newTestService(t)
	// data field is int -- unsupported by toBytes; should log but not panic.
	svc.HandlePaneOutputEvent(map[string]any{
		"paneId": "%0",
		"data":   12345,
	})
}

// ---------------------------------------------------------------------------
// StopOutputBuffer
// ---------------------------------------------------------------------------

func TestStopOutputBuffer(t *testing.T) {
	var removedPanes sync.Map
	d := validDeps()
	d.HasPaneStates = func() bool { return true }
	d.PaneStateRemovePane = func(paneID string) {
		removedPanes.Store(paneID, true)
	}
	svc := NewService(d)
	t.Cleanup(func() { svc.Shutdown() })

	// Trigger output so flusher is created.
	svc.HandlePaneOutputEvent(&tmux.PaneOutputEvent{PaneID: "%5", Data: []byte("data")})
	svc.StopOutputBuffer("%5")

	if _, ok := removedPanes.Load("%5"); !ok {
		t.Error("PaneStateRemovePane was not called for %5")
	}
}

// ---------------------------------------------------------------------------
// DetachAllOutputBuffers
// ---------------------------------------------------------------------------

func TestDetachOutputBuffers(t *testing.T) {
	svc := newTestService(t)

	// No flusher yet -> should return nil.
	removed := svc.DetachAllOutputBuffers()
	if removed != nil {
		t.Errorf("expected nil from DetachAllOutputBuffers with no flusher, got %v", removed)
	}

	// Trigger output to create the flusher then detach.
	svc.HandlePaneOutputEvent(&tmux.PaneOutputEvent{PaneID: "%0", Data: []byte("x")})
	removed = svc.DetachAllOutputBuffers()
	// We expect at least pane %0 in the removed list.
	if !slices.Contains(removed, "%0") {
		t.Errorf("expected %%0 in removed list, got %v", removed)
	}
}

// ---------------------------------------------------------------------------
// PaneFeedWorker lifecycle
// ---------------------------------------------------------------------------

func TestStartPaneFeedWorkerLifecycle(t *testing.T) {
	var feedCalled atomic.Int32

	d := validDeps()
	d.HasPaneStates = func() bool { return true }
	d.PaneStateFeedTrimmed = func(_ string, _ []byte) {
		feedCalled.Add(1)
	}
	var bgWG sync.WaitGroup
	d.LaunchWorker = func(name string, ctx context.Context, fn func(ctx context.Context), opts workerutil.RecoveryOptions) {
		workerutil.RunWithPanicRecovery(ctx, name, &bgWG, fn, opts)
	}
	d.BaseRecoveryOptions = func() workerutil.RecoveryOptions {
		return workerutil.RecoveryOptions{MaxRetries: 1}
	}
	svc := NewService(d)

	ctx, cancel := context.WithCancel(context.Background())
	svc.StartPaneFeedWorker(ctx)

	// Enqueue an item.
	svc.enqueuePaneStateFeed("%0", []byte("abc"))

	waitForCondition(t, 500*time.Millisecond, func() bool {
		return feedCalled.Load() > 0
	}, "feed worker should have processed the item")

	cancel()
	bgWG.Wait()
}

func TestStopPaneFeedWorker(t *testing.T) {
	svc := newTestService(t)

	ctx := context.Background()
	svc.StartPaneFeedWorker(ctx)
	svc.StopPaneFeedWorker()

	// Double stop must not panic.
	svc.StopPaneFeedWorker()
}

// ---------------------------------------------------------------------------
// enqueuePaneStateFeed
// ---------------------------------------------------------------------------

func TestEnqueuePaneStateFeed(t *testing.T) {
	t.Run("hasPaneStates_false_is_noop", func(t *testing.T) {
		d := validDeps()
		d.HasPaneStates = func() bool { return false }
		svc := NewService(d)
		// Must not panic or block.
		svc.enqueuePaneStateFeed("%0", []byte("data"))
	})

	t.Run("empty_chunk_is_noop", func(t *testing.T) {
		svc := newTestService(t)
		// Must not panic.
		svc.enqueuePaneStateFeed("%0", nil)
		svc.enqueuePaneStateFeed("%0", []byte{})
	})
}

// ---------------------------------------------------------------------------
// T-1: Channel full fallback path
// ---------------------------------------------------------------------------

func TestEnqueuePaneStateFeed_ChannelFull_FallbackToDirectFeed(t *testing.T) {
	var feedCount atomic.Int32
	d := validDeps()
	d.HasPaneStates = func() bool { return true }
	d.PaneStateFeedTrimmed = func(_ string, _ []byte) {
		feedCount.Add(1)
	}
	svc := NewService(d)
	t.Cleanup(func() { svc.Shutdown() })

	// Fill the channel without starting a worker (no consumer).
	for range paneFeedChSize {
		svc.enqueuePaneStateFeed("%0", []byte("fill"))
	}

	// The next enqueue must use the direct-feed fallback.
	svc.enqueuePaneStateFeed("%0", []byte("overflow"))

	if feedCount.Load() != 1 {
		t.Errorf("expected exactly 1 direct feed call (fallback), got %d", feedCount.Load())
	}
}

// ---------------------------------------------------------------------------
// T-3: DetachStaleOutputBuffers
// ---------------------------------------------------------------------------

func TestDetachStaleOutputBuffers(t *testing.T) {
	t.Run("nil_flusher_returns_nil", func(t *testing.T) {
		svc := newTestService(t)
		got := svc.DetachStaleOutputBuffers(map[string]struct{}{"%0": {}})
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("removes_stale_panes", func(t *testing.T) {
		svc := newTestService(t)

		// Create output buffers for 2 panes.
		svc.HandlePaneOutputEvent(&tmux.PaneOutputEvent{PaneID: "%0", Data: []byte("a")})
		svc.HandlePaneOutputEvent(&tmux.PaneOutputEvent{PaneID: "%1", Data: []byte("b")})

		// Retain only %0; %1 should be removed.
		removed := svc.DetachStaleOutputBuffers(map[string]struct{}{"%0": {}})
		if !slices.Contains(removed, "%1") {
			t.Errorf("expected %%1 in removed list, got %v", removed)
		}
		if slices.Contains(removed, "%0") {
			t.Errorf("%%0 should NOT be in removed list, got %v", removed)
		}
	})
}

// ---------------------------------------------------------------------------
// IsPaneQuiet
// ---------------------------------------------------------------------------

func TestIsPaneQuiet(t *testing.T) {
	t.Run("nil_flusher_returns_true", func(t *testing.T) {
		svc := newTestService(t)
		// No output has been sent, so no flusher exists.
		if !svc.IsPaneQuiet("%0") {
			t.Error("expected IsPaneQuiet to return true when flusher is nil")
		}
	})

	t.Run("unknown_pane_returns_true", func(t *testing.T) {
		svc := newTestService(t)
		// Create flusher by sending output to a different pane.
		svc.HandlePaneOutputEvent(&tmux.PaneOutputEvent{PaneID: "%0", Data: []byte("x")})
		// Query an unknown pane — should return true (no state tracked).
		if !svc.IsPaneQuiet("%999") {
			t.Error("expected IsPaneQuiet to return true for unknown pane")
		}
	})

	t.Run("recent_output_returns_false", func(t *testing.T) {
		svc := newTestService(t)
		svc.HandlePaneOutputEvent(&tmux.PaneOutputEvent{PaneID: "%0", Data: []byte("data")})
		// Output was just sent — pane should be busy.
		if svc.IsPaneQuiet("%0") {
			t.Error("expected IsPaneQuiet to return false immediately after output")
		}
	})

	t.Run("stale_output_returns_true", func(t *testing.T) {
		svc := newTestService(t)
		svc.HandlePaneOutputEvent(&tmux.PaneOutputEvent{PaneID: "%0", Data: []byte("data")})

		// Wait past the quiet threshold (3 seconds + margin).
		waitForCondition(t, 4*time.Second, func() bool {
			return svc.IsPaneQuiet("%0")
		}, "expected pane to become quiet after threshold elapsed")
	})
}
