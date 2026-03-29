package snapshot

// output.go — Pane output buffering, flush management, and pane feed worker.
//
// Methods in this file handle the high-frequency PTY output path:
//   - HandlePaneOutputEvent dispatches incoming events to the enqueue pipeline.
//   - ensureOutputFlusher lazily initializes the OutputFlushManager.
//   - Detach*/Stop* methods manage output buffer lifecycle.
//   - StartPaneFeedWorker/StopPaneFeedWorker manage the async pane state feed goroutine.

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"myT-x/internal/terminal"
)

const (
	// outputFlushInterval is the maximum time between output chunk flushes to
	// the frontend. Chosen to match a 60 fps frame budget (~16 ms).
	outputFlushInterval = 16 * time.Millisecond
	// outputFlushThreshold is the per-pane write buffer flush threshold in OutputFlushManager.
	// 32 KiB balances IPC payload size against flush frequency: at 1,000 tokens/sec
	// (~6 KB/sec/pane), a 32 KiB buffer fills in ~5 sec under sustained single-pane
	// output, reducing wakeCh signal frequency by ~4x compared to 8 KiB.
	outputFlushThreshold = 32 * 1024

	// paneFeedChSize is the buffer size for the pane feed channel.
	// 4096 items provides ~4 seconds of headroom at 10 panes x 100 chunks/sec.
	// When full, enqueuePaneStateFeed falls back to direct Feed() call.
	paneFeedChSize = 4096
)

const paneQuietThreshold = 3 * time.Second

// IsPaneQuiet returns true when the specified pane has had no terminal output
// for the quiet threshold duration (3 seconds). Used by the scheduler to avoid
// sending messages while an AI agent is actively generating output.
func (s *Service) IsPaneQuiet(paneID string) bool {
	s.outputMu.Lock()
	flusher := s.outputFlusher
	s.outputMu.Unlock()

	if flusher == nil {
		slog.Debug("[DEBUG-SNAPSHOT] IsPaneQuiet: no flusher, treating as quiet", "paneID", paneID)
		return true // no flusher yet → no output has ever been tracked
	}
	return flusher.IsPaneQuiet(paneID, paneQuietThreshold)
}

// HandlePaneOutputEvent processes a tmux:pane-output event, dispatching to the
// appropriate enqueue path based on payload type.
func (s *Service) HandlePaneOutputEvent(payload any) {
	if s.shutdownCalled.Load() {
		return
	}
	if evt, handled := paneOutputEventFromPayload(payload); handled {
		if evt == nil {
			// Explicit nil pointer payload is treated as a no-op for backward compatibility.
			slog.Debug("[EVENT] pane-output: nil PaneOutputEvent payload ignored")
			return
		}
		paneID := strings.TrimSpace(evt.PaneID)
		if paneID != "" && len(evt.Data) > 0 {
			s.enqueuePaneOutput(paneID, evt.Data)
		}
		return
	}

	if data, ok := payload.(map[string]any); ok {
		s.handleLegacyMapPaneOutput(data)
		return
	}

	// Log only the type; fmt.Sprintf("%v") on arbitrary types can panic
	// if a String()/GoString() method dereferences a nil receiver.
	slog.Warn("[EVENT] pane-output: unexpected payload type",
		"type", fmt.Sprintf("%T", payload))
}

// handleLegacyMapPaneOutput processes a legacy map[string]any pane-output payload.
// TODO: Remove after frontend/backends are fully aligned on PaneOutputEvent.
func (s *Service) handleLegacyMapPaneOutput(data map[string]any) {
	paneID, chunk := parseLegacyMapPaneOutput(data)
	if chunk == nil && data["data"] != nil {
		// toBytes returns nil for unsupported types (e.g. int, struct).
		// Log the type mismatch to assist debugging legacy callers.
		slog.Debug("[EVENT] legacy pane-output: unsupported data field type",
			"dataType", fmt.Sprintf("%T", data["data"]),
			"paneId", paneID)
		return
	}
	if paneID == "" || len(chunk) == 0 {
		// Empty pane output can occur around startup/transition boundaries.
		slog.Debug("[EVENT] skip empty pane-output payload",
			"paneId", paneID,
			"chunkLen", len(chunk))
		return
	}
	s.enqueuePaneOutput(paneID, chunk)
}

func (s *Service) enqueuePaneOutput(paneID string, chunk []byte) {
	// Hot path: avoid SessionManager lock on every chunk.
	// Stale pane cleanup is handled by StopOutputBuffer + snapshot reconciliation.
	slog.Debug("[output] enqueuePaneOutput", "paneId", paneID, "chunkLen", len(chunk))
	s.enqueuePaneStateFeed(paneID, chunk)
	flusher := s.ensureOutputFlusher()
	flusher.Write(paneID, chunk)
}

func (s *Service) ensureOutputFlusher() *terminal.OutputFlushManager {
	s.outputMu.Lock()
	defer s.outputMu.Unlock()

	if s.outputFlusher != nil {
		return s.outputFlusher
	}
	flusher := terminal.NewOutputFlushManager(outputFlushInterval, outputFlushThreshold, func(paneID string, flushed []byte) {
		if len(flushed) == 0 {
			return
		}
		ctx := s.deps.RuntimeContext()
		if ctx == nil {
			slog.Debug("[output] skip pane flush because runtime context is nil", "paneId", paneID)
			return
		}
		if s.deps.UpdateActivityByPaneID(paneID) {
			s.RequestSnapshot(false)
		}
		// Delivery strategy (WebSocket vs IPC) is encapsulated in the dep closure.
		s.deps.DeliverPaneOutput(ctx, paneID, flushed)
	})
	flusher.Start()
	s.outputFlusher = flusher
	return flusher
}

// DetachAllOutputBuffers detaches all tracked pane output buffers and returns pane IDs
// for pane-state cleanup.
func (s *Service) DetachAllOutputBuffers() []string {
	s.outputMu.Lock()
	flusher := s.outputFlusher
	s.outputFlusher = nil
	s.outputMu.Unlock()
	if flusher == nil {
		return nil
	}
	removed := flusher.RetainPanes(nil)
	flusher.Stop()
	return removed
}

// DetachStaleOutputBuffers removes buffers for panes that no longer exist and
// returns removed pane IDs for pane-state cleanup.
//
// The lock is held for the full operation to prevent a concurrent
// DetachAllOutputBuffers from stopping the flusher mid-retain (TOCTOU).
func (s *Service) DetachStaleOutputBuffers(existingPanes map[string]struct{}) []string {
	s.outputMu.Lock()
	defer s.outputMu.Unlock()
	if s.outputFlusher == nil {
		return nil
	}
	return s.outputFlusher.RetainPanes(existingPanes)
}

// CleanupDetachedPaneStates removes corresponding pane state entries.
func (s *Service) CleanupDetachedPaneStates(paneIDs []string) {
	for _, paneID := range paneIDs {
		s.deps.PaneStateRemovePane(paneID)
	}
}

// StopOutputBuffer removes the output buffer and pane state for a single pane.
//
// The lock is held for the flusher operation to prevent a concurrent
// DetachAllOutputBuffers from stopping the flusher mid-remove (TOCTOU).
func (s *Service) StopOutputBuffer(paneID string) {
	s.outputMu.Lock()
	if s.outputFlusher != nil {
		s.outputFlusher.RemovePane(paneID)
	}
	s.outputMu.Unlock()
	s.deps.PaneStateRemovePane(paneID)
}

// StartPaneFeedWorker launches the pane feed goroutine.
func (s *Service) StartPaneFeedWorker(parent context.Context) {
	if !s.deps.HasPaneStates() || s.paneFeedCh == nil {
		return
	}

	ctx, cancel := context.WithCancel(parent)
	s.outputMu.Lock()
	s.paneFeedStop = cancel
	s.outputMu.Unlock()

	ch := s.paneFeedCh
	feedFn := s.deps.PaneStateFeedTrimmed

	opts := s.deps.BaseRecoveryOptions()
	prevOnFatal := opts.OnFatal
	opts.OnFatal = func(worker string, maxRetries int) {
		slog.Warn("[FEED] pane-feed-worker permanently stopped after max retries; "+
			"new feed items will use direct-call fallback",
			"worker", worker, "maxRetries", maxRetries)
		if prevOnFatal != nil {
			prevOnFatal(worker, maxRetries)
		}
	}

	s.deps.LaunchWorker("pane-feed-worker", ctx, func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case item := <-ch:
				// NOTE: If FeedTrimmed panics, putFeedBuffer will not be called
				// and the pool buffer will leak (one per panic). GC will eventually
				// collect it, so this is acceptable.
				// feedFn is guaranteed non-nil (defaulted in NewService).
				feedFn(item.paneID, item.chunk)
				putFeedBuffer(item.poolPtr)
			}
		}
	}, opts)
}

// StopPaneFeedWorker cancels the pane feed goroutine and drains the channel.
func (s *Service) StopPaneFeedWorker() {
	s.outputMu.Lock()
	cancel := s.paneFeedStop
	s.paneFeedStop = nil
	s.outputMu.Unlock()
	if cancel != nil {
		cancel()
	}
	// Best-effort drain so pooled buffers are returned promptly after stop.
	for {
		select {
		case item := <-s.paneFeedCh:
			putFeedBuffer(item.poolPtr)
		default:
			return
		}
	}
}

func (s *Service) enqueuePaneStateFeed(paneID string, chunk []byte) {
	if !s.deps.HasPaneStates() || len(chunk) == 0 {
		return
	}
	pooled, bp := getFeedBuffer(len(chunk))
	copy(pooled, chunk)
	item := paneFeedItem{
		paneID:  paneID,
		chunk:   pooled,
		poolPtr: bp,
	}
	// NOTE: When the channel is full, the direct-feed fallback bypasses the worker,
	// so ordering between the fallback chunk and the worker's next dequeue is not
	// guaranteed. Additionally, concurrent FeedTrimmed calls from different goroutines
	// (worker vs. fallback) are safe because panestate.FeedTrimmed is internally
	// synchronized (RLock/Lock). This is acceptable because xterm.js buffers and
	// renders output asynchronously; minor reordering within a single flush cycle
	// is invisible to the user and does not corrupt terminal state.
	select {
	case s.paneFeedCh <- item:
	default:
		slog.Debug("[DEBUG-FEED] channel full, falling back to direct feed", "paneId", paneID, "chunkLen", len(chunk))
		// PaneStateFeedTrimmed is guaranteed non-nil (defaulted in NewService).
		s.deps.PaneStateFeedTrimmed(paneID, pooled)
		putFeedBuffer(bp)
	}
}
