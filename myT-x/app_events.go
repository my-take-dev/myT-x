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

const snapshotCoalesceWindow = 50 * time.Millisecond

// emitRuntimeEvent emits via the app context and delegates to emitRuntimeEventWithContext.
func (a *App) emitRuntimeEvent(name string, payload any) {
	a.emitRuntimeEventWithContext(a.runtimeContext(), name, payload)
}

// emitRuntimeEventWithContext emits a runtime event only when ctx is non-nil.
// Prefer this helper for best-effort contexts that may not be initialized yet.
func (a *App) emitRuntimeEventWithContext(ctx context.Context, name string, payload any) {
	if ctx == nil {
		slog.Warn("[DEBUG-EVENT] runtime event dropped because app context is nil", "event", name)
		return
	}
	runtimeEventsEmitFn(ctx, name, payload)
}

// emitBackendEvent handles backend-originated runtime events.
// For tmux pane output, it also buffers chunks and emits pane snapshots.
func (a *App) emitBackendEvent(name string, payload any) {
	ctx := a.runtimeContext()
	if ctx == nil {
		slog.Warn("[DEBUG-EVENT] backend event dropped because runtime context is nil", "event", name)
		return
	}
	if name == "app:activate-window" {
		a.bringWindowToFront()
		return
	}

	if name == "tmux:pane-output" {
		if evt, handled := paneOutputEventFromPayload(payload); handled {
			if evt == nil {
				// Explicit nil pointer payload is treated as a no-op for backward compatibility.
				slog.Debug("[DEBUG-EVENT] pane-output: nil PaneOutputEvent payload ignored")
				return
			}
			paneID := strings.TrimSpace(evt.PaneID)
			if paneID != "" && len(evt.Data) > 0 {
				a.enqueuePaneOutput(paneID, evt.Data)
			}
			return
		}

		if data, ok := payload.(map[string]any); ok {
			// TODO: Remove legacy map payload support after frontend/backends are fully aligned on PaneOutputEvent.
			paneID := strings.TrimSpace(toString(data["paneId"]))
			chunk := toBytes(data["data"])
			if paneID == "" || len(chunk) == 0 {
				// Empty pane output can occur around startup/transition boundaries.
				slog.Debug("[DEBUG-EVENT] skip empty pane-output payload",
					"paneId", paneID,
					"chunkLen", len(chunk))
				return
			}
			a.enqueuePaneOutput(paneID, chunk)
			return
		}

		slog.Warn("[DEBUG-EVENT] pane-output: unexpected payload type",
			"type", fmt.Sprintf("%T", payload),
			"rawPayload", fmt.Sprintf("%v", payload))
		return
	}

	a.emitRuntimeEventWithContext(ctx, name, payload)
	if shouldEmitSnapshotForEvent(name) {
		a.requestSnapshot(shouldBypassSnapshotDebounceForEvent(name))
	}
}

func shouldEmitSnapshotForEvent(name string) bool {
	switch name {
	case "tmux:session-created",
		"tmux:session-destroyed",
		"tmux:pane-created",
		"tmux:layout-changed",
		"tmux:pane-focused",
		"tmux:pane-renamed":
		return true
	default:
		return false
	}
}

func shouldBypassSnapshotDebounceForEvent(name string) bool {
	switch name {
	case "tmux:session-created", "tmux:session-destroyed", "tmux:pane-focused", "tmux:pane-renamed":
		return true
	default:
		return false
	}
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
	flusher := terminal.NewOutputFlushManager(16*time.Millisecond, 8*1024, func(paneID string, flushed []byte) {
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
		slog.Debug("[output] flushing to frontend", "paneId", paneID, "flushedLen", len(flushed))
		a.emitRuntimeEventWithContext(ctx, "pane:data:"+paneID, string(flushed))
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
