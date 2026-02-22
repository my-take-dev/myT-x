package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"myT-x/internal/terminal"
	"myT-x/internal/tmux"
)

const (
	// snapshotCoalesceWindow is the debounce window for coalescing snapshot
	// event emissions. 50 ms balances UI responsiveness (structural changes
	// visible within one frame budget) against CPU savings from batching
	// output-driven snapshot updates. For high-parallelism deployments
	// (10+ panes), 100 ms may further reduce load.
	// TODO(perf): Make configurable via AppConfig.snapshot_coalesce_ms.
	snapshotCoalesceWindow = 50 * time.Millisecond
	// outputFlushInterval is the maximum time between output chunk flushes to
	// the frontend. Chosen to match a 60 fps frame budget (~16 ms).
	outputFlushInterval = 16 * time.Millisecond
	// outputFlushThreshold is the per-pane write buffer flush threshold in OutputFlushManager.
	// 32 KiB balances IPC payload size against flush frequency: at 1,000 tokens/sec
	// (~6 KB/sec/pane), a 32 KiB buffer fills in ~5 sec under sustained single-pane
	// output, reducing wakeCh signal frequency by ~4x compared to 8 KiB.
	outputFlushThreshold = 32 * 1024
)

// emitRuntimeEvent emits via the app context and delegates to emitRuntimeEventWithContext.
func (a *App) emitRuntimeEvent(name string, payload any) {
	a.emitRuntimeEventWithContext(a.runtimeContext(), name, payload)
}

// emitRuntimeEventWithContext emits a runtime event only when ctx is non-nil.
// Prefer this helper for best-effort contexts that may not be initialized yet.
func (a *App) emitRuntimeEventWithContext(ctx context.Context, name string, payload any) {
	if ctx == nil {
		slog.Warn("[EVENT] runtime event dropped because app context is nil", "event", name)
		return
	}
	runtimeEventsEmitFn(ctx, name, payload)
}

// emitBackendEvent handles backend-originated runtime events.
// For tmux pane output, it also buffers chunks and emits pane snapshots.
func (a *App) emitBackendEvent(name string, payload any) {
	ctx := a.runtimeContext()
	if ctx == nil {
		slog.Warn("[EVENT] backend event dropped because runtime context is nil", "event", name)
		return
	}
	if name == "app:activate-window" {
		a.bringWindowToFront()
		return
	}
	if name == "tmux:pane-output" {
		a.handlePaneOutputEvent(payload)
		return
	}

	a.emitRuntimeEventWithContext(ctx, name, payload)
	if shouldEmitSnapshotForEvent(name) {
		a.requestSnapshot(shouldBypassSnapshotDebounceForEvent(name))
	}
}

// handlePaneOutputEvent processes a tmux:pane-output event, dispatching to the
// appropriate enqueue path based on payload type.
func (a *App) handlePaneOutputEvent(payload any) {
	if evt, handled := paneOutputEventFromPayload(payload); handled {
		if evt == nil {
			// Explicit nil pointer payload is treated as a no-op for backward compatibility.
			slog.Debug("[EVENT] pane-output: nil PaneOutputEvent payload ignored")
			return
		}
		paneID := strings.TrimSpace(evt.PaneID)
		if paneID != "" && len(evt.Data) > 0 {
			a.enqueuePaneOutput(paneID, evt.Data)
		}
		return
	}

	if data, ok := payload.(map[string]any); ok {
		a.handleLegacyMapPaneOutput(data)
		return
	}

	slog.Warn("[EVENT] pane-output: unexpected payload type",
		"type", fmt.Sprintf("%T", payload),
		"rawPayload", fmt.Sprintf("%v", payload))
}

// handleLegacyMapPaneOutput processes a legacy map[string]any pane-output payload.
// TODO: Remove after frontend/backends are fully aligned on PaneOutputEvent.
func (a *App) handleLegacyMapPaneOutput(data map[string]any) {
	paneID := strings.TrimSpace(toString(data["paneId"]))
	rawData := data["data"]
	chunk := toBytes(rawData)
	if chunk == nil && rawData != nil {
		// toBytes returns nil for unsupported types (e.g. int, struct).
		// Log the type mismatch to assist debugging legacy callers.
		slog.Debug("[EVENT] legacy pane-output: unsupported data field type",
			"dataType", fmt.Sprintf("%T", rawData),
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
	a.enqueuePaneOutput(paneID, chunk)
}

// snapshotEventPolicy defines how a backend event interacts with snapshot emission.
type snapshotEventPolicy struct {
	// trigger indicates that the event should trigger a snapshot emission.
	trigger bool
	// bypassDebounce indicates that the snapshot should bypass the coalesce debounce window.
	bypassDebounce bool
}

// snapshotEventPolicies maps backend event names to their snapshot emission behavior.
// Adding a new event that requires snapshot emission only needs a single entry here.
//
// DESIGN: All current entries have trigger=true. shouldEmitSnapshotForEvent checks
// both key existence and trigger==true, so adding a trigger=false entry is safe.
//
// NOTE(1-window model): Currently unreachable - retained for future multi-window support.
// When re-enabling, also restore tmux:window-created/renamed/destroyed event emissions.
// The current architecture enforces 1 session = 1 window; window-level events
// (tmux:window-destroyed, tmux:window-renamed) are never emitted at runtime because
// a single window always exists per session and the session snapshot already contains
// all window information. The policy entries below are kept so that a future
// multi-window extension only needs to start emitting the events - no policy
// registration change is required.
//
// INVARIANT: immutable after init - do not modify at runtime.
var snapshotEventPolicies = map[string]snapshotEventPolicy{
	"tmux:session-created":   {trigger: true, bypassDebounce: true},
	"tmux:session-destroyed": {trigger: true, bypassDebounce: true},
	"tmux:session-renamed":   {trigger: true, bypassDebounce: true},
	"tmux:pane-created":      {trigger: true, bypassDebounce: false},
	"tmux:layout-changed":    {trigger: true, bypassDebounce: false},
	"tmux:pane-focused":      {trigger: true, bypassDebounce: true},
	"tmux:pane-renamed":      {trigger: true, bypassDebounce: true},
	// NOTE(1-window model): Policy is registered for future multi-window support.
	// No runtime emitter currently exists for tmux:window-created.
	"tmux:window-created": {trigger: true, bypassDebounce: true},
	// NOTE(1-window model): Window-level lifecycle events remain registered for
	// future multi-window support but are not emitted in normal runtime.
	"tmux:window-destroyed": {trigger: true, bypassDebounce: true},
	"tmux:window-renamed":   {trigger: true, bypassDebounce: true},
}

func shouldEmitSnapshotForEvent(name string) bool {
	policy, ok := snapshotEventPolicies[name]
	return ok && policy.trigger
}

func shouldBypassSnapshotDebounceForEvent(name string) bool {
	policy, ok := snapshotEventPolicies[name]
	return ok && policy.bypassDebounce
}

func paneOutputEventFromPayload(payload any) (*tmux.PaneOutputEvent, bool) {
	switch event := payload.(type) {
	case *tmux.PaneOutputEvent:
		if event == nil {
			return nil, true
		}
		return event, true
	case tmux.PaneOutputEvent:
		copied := event
		copied.Data = append([]byte(nil), event.Data...)
		return &copied, true
	default:
		return nil, false
	}
}

func (a *App) enqueuePaneOutput(paneID string, chunk []byte) {
	// Hot path: avoid SessionManager lock on every chunk.
	// Stale pane cleanup is handled by stopOutputBuffer + snapshot reconciliation.
	slog.Debug("[output] enqueuePaneOutput", "paneId", paneID, "chunkLen", len(chunk))
	a.enqueuePaneStateFeed(paneID, chunk)
	flusher := a.ensureOutputFlusher()
	flusher.Write(paneID, chunk)
}

func (a *App) ensureOutputFlusher() *terminal.OutputFlushManager {
	a.outputMu.Lock()
	defer a.outputMu.Unlock()

	if a.outputFlusher != nil {
		return a.outputFlusher
	}
	flusher := terminal.NewOutputFlushManager(outputFlushInterval, outputFlushThreshold, func(paneID string, flushed []byte) {
		if len(flushed) == 0 {
			return
		}
		ctx := a.runtimeContext()
		if ctx == nil {
			slog.Debug("[output] skip pane flush because runtime context is nil", "paneId", paneID)
			return
		}
		if sessions := a.sessions; sessions != nil && sessions.UpdateActivityByPaneID(paneID) {
			a.requestSnapshot(false)
		}
		// Prefer WebSocket binary stream for pane data (avoids Wails IPC JSON overhead).
		// Falls back to Wails IPC when no WebSocket client is connected (e.g. during
		// startup before frontend establishes the WebSocket channel).
		//
		// NOTE (TOCTOU): HasActiveConnection() and BroadcastPaneData are not atomic.
		// If the WebSocket closes between this check and BroadcastPaneData(),
		// BroadcastPaneData returns an error and clears the connection; the data
		// for this flush window (<= outputFlushInterval = 16 ms) is lost.
		// This is an accepted design trade-off: the frontend reconnects via
		// paneDataStream's exponential backoff, and any missed terminal output
		// is at most one flush interval worth of data - invisible to users.
		if a.wsHub != nil && a.wsHub.HasActiveConnection() {
			a.wsHub.BroadcastPaneData(paneID, flushed)
		} else {
			slog.Debug("[output] flushing to frontend via Wails IPC", "paneId", paneID, "flushedLen", len(flushed))
			a.emitRuntimeEventWithContext(ctx, "pane:data:"+paneID, string(flushed))
		}
	})
	flusher.Start()
	a.outputFlusher = flusher
	return flusher
}

// detachAllOutputBuffers detaches all tracked pane output buffers and returns pane IDs
// for pane-state cleanup.
func (a *App) detachAllOutputBuffers() []string {
	a.outputMu.Lock()
	flusher := a.outputFlusher
	a.outputFlusher = nil
	a.outputMu.Unlock()
	if flusher == nil {
		return nil
	}
	removed := flusher.RetainPanes(nil)
	flusher.Stop()
	return removed
}

// detachStaleOutputBuffers removes buffers for panes that no longer exist and
// returns removed pane IDs for pane-state cleanup.
func (a *App) detachStaleOutputBuffers(existingPanes map[string]struct{}) []string {
	a.outputMu.Lock()
	flusher := a.outputFlusher
	a.outputMu.Unlock()
	if flusher == nil {
		return nil
	}
	return flusher.RetainPanes(existingPanes)
}

// cleanupDetachedPaneStates removes corresponding pane state entries.
func (a *App) cleanupDetachedPaneStates(paneIDs []string) {
	if a.paneStates == nil {
		return
	}
	for _, paneID := range paneIDs {
		a.paneStates.RemovePane(paneID)
	}
}

func (a *App) stopOutputBuffer(paneID string) {
	a.outputMu.Lock()
	flusher := a.outputFlusher
	a.outputMu.Unlock()
	if flusher != nil {
		flusher.RemovePane(paneID)
	}
	if a.paneStates != nil {
		a.paneStates.RemovePane(paneID)
	}
}

func (a *App) emitSnapshot() {
	ctx := a.runtimeContext()
	if ctx == nil {
		slog.Debug("[snapshot] skip emitSnapshot: runtime context is nil")
		return
	}
	// SessionManager pointer is initialized once at startup and kept stable.
	// During shutdown it is closed but not replaced, so a local snapshot is safe.
	sessions := a.sessions
	if sessions == nil {
		slog.Debug("[snapshot] skip emitSnapshot: sessions manager is nil")
		return
	}
	snapshots := sessions.Snapshot()
	if a.shouldSyncPaneStates(sessions.TopologyGeneration()) {
		a.syncPaneStates(snapshots)
	}
	delta, changed, initial := a.snapshotDelta(snapshots)
	if initial {
		a.emitRuntimeEventWithContext(ctx, "tmux:snapshot", snapshots)
		a.recordSnapshotEmission("full", a.estimateSnapshotPayloadBytes(snapshots))
		return
	}
	if !changed {
		return
	}
	a.emitRuntimeEventWithContext(ctx, "tmux:snapshot-delta", delta)
	a.recordSnapshotEmission("delta", a.estimateSnapshotPayloadBytes(delta))
}

// shouldSyncPaneStates tracks the last synced topology generation and returns
// whether pane state synchronization should run for this snapshot.
// NOTE: This predicate has side effects by updating snapshotLastTopology.
func (a *App) shouldSyncPaneStates(topologyGeneration uint64) bool {
	a.snapshotMu.Lock()
	defer a.snapshotMu.Unlock()
	if a.snapshotLastTopology == topologyGeneration {
		return false
	}
	a.snapshotLastTopology = topologyGeneration
	return true
}

func (a *App) syncPaneStates(snapshots []tmux.SessionSnapshot) {
	if a.paneStates == nil {
		return
	}

	alive := make(map[string]struct{})
	active := make(map[string]struct{})
	for _, session := range snapshots {
		for _, window := range session.Windows {
			if window.Panes == nil {
				continue
			}
			for _, pane := range window.Panes {
				alive[pane.ID] = struct{}{}
				if pane.Active {
					active[pane.ID] = struct{}{}
				}
				a.paneStates.EnsurePane(pane.ID, pane.Width, pane.Height)
			}
		}
	}
	a.paneStates.SetActivePanes(active)
	a.paneStates.RetainPanes(alive)
}

func (a *App) requestSnapshot(immediate bool) {
	if a.runtimeContext() == nil {
		slog.Debug("[snapshot] skip requestSnapshot: runtime context is nil")
		return
	}
	// App.sessions is assigned once during startup and not set back to nil during
	// normal runtime, so this lock-free nil guard is safe on the hot path.
	if a.sessions == nil {
		slog.Debug("[snapshot] skip requestSnapshot: sessions manager is nil")
		return
	}

	// Strategy: leading-edge fixed-window debounce.
	//
	// immediate=true (structural events: session/pane/window created/destroyed):
	//   Cancels any pending timer and emits synchronously on the caller's goroutine.
	//   The generation counter prevents duplicate emissions when multiple callers
	//   race into the immediate path simultaneously.
	//
	// immediate=false (output-driven activity updates):
	//   Arms a one-shot timer for snapshotCoalesceWindow. If another request
	//   arrives before the timer fires, the timer is left running (no reset);
	//   this bounds the worst-case delay to one coalesce window regardless of
	//   how many events arrive.
	emitNow := false
	a.snapshotRequestMu.Lock()
	a.snapshotRequestGeneration++
	currentGeneration := a.snapshotRequestGeneration

	if immediate {
		if a.snapshotRequestTimer != nil {
			a.snapshotRequestTimer.Stop()
			a.snapshotRequestTimer = nil
		}
		if a.snapshotRequestDispatched < currentGeneration {
			a.snapshotRequestDispatched = currentGeneration
			emitNow = true
		}
		a.snapshotRequestMu.Unlock()
		if emitNow {
			a.emitSnapshot()
		}
		return
	}

	if a.snapshotRequestTimer == nil {
		a.snapshotRequestTimer = time.AfterFunc(snapshotCoalesceWindow, a.flushSnapshotRequest)
	}
	a.snapshotRequestMu.Unlock()
}

func (a *App) flushSnapshotRequest() {
	ctx := a.runtimeContext()
	sessions := a.sessions
	emitNow := false

	a.snapshotRequestMu.Lock()
	a.snapshotRequestTimer = nil
	if ctx == nil {
		a.snapshotRequestMu.Unlock()
		slog.Debug("[snapshot] skip flushSnapshotRequest: runtime context is nil")
		return
	}
	if sessions == nil {
		a.snapshotRequestMu.Unlock()
		slog.Debug("[snapshot] skip flushSnapshotRequest: sessions manager is nil")
		return
	}
	if a.snapshotRequestDispatched < a.snapshotRequestGeneration {
		a.snapshotRequestDispatched = a.snapshotRequestGeneration
		emitNow = true
	}
	a.snapshotRequestMu.Unlock()

	if emitNow {
		a.emitSnapshot()
	}
}

func (a *App) clearSnapshotRequestTimer() {
	a.snapshotRequestMu.Lock()
	if a.snapshotRequestTimer != nil {
		a.snapshotRequestTimer.Stop()
		a.snapshotRequestTimer = nil
	}
	a.snapshotRequestMu.Unlock()
}
