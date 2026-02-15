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
	if a.runtimeContext() == nil {
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

	a.emitRuntimeEvent(name, payload)
	if shouldEmitSnapshotForEvent(name) {
		a.emitSnapshot()
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
	buffer := a.ensureOutputBuffer(paneID)
	buffer.Write(chunk)
}

func (a *App) ensureOutputBuffer(paneID string) *terminal.OutputBuffer {
	a.outputMu.Lock()
	defer a.outputMu.Unlock()

	if buffer, ok := a.outputBuffers[paneID]; ok {
		return buffer
	}
	sessions := a.sessions
	buffer := terminal.NewOutputBuffer(16*time.Millisecond, 8*1024, func(flushed []byte) {
		if len(flushed) == 0 {
			return
		}
		ctx := a.runtimeContext()
		if ctx == nil {
			return
		}
		if sessions != nil && sessions.UpdateActivityByPaneID(paneID) {
			a.emitSnapshot()
		}
		slog.Debug("[output] flushing to frontend", "paneId", paneID, "flushedLen", len(flushed))
		a.emitRuntimeEventWithContext(ctx, "pane:data:"+paneID, string(flushed))
	})
	buffer.Start()
	a.outputBuffers[paneID] = buffer
	return buffer
}

// detachAllOutputBuffers removes all tracked pane output buffers from the app map
// and returns them for lock-free stopping.
func (a *App) detachAllOutputBuffers() map[string]*terminal.OutputBuffer {
	a.outputMu.Lock()
	if len(a.outputBuffers) == 0 {
		a.outputMu.Unlock()
		return nil
	}
	detached := make(map[string]*terminal.OutputBuffer, len(a.outputBuffers))
	for paneID, buffer := range a.outputBuffers {
		detached[paneID] = buffer
	}
	a.outputBuffers = map[string]*terminal.OutputBuffer{}
	a.outputMu.Unlock()
	return detached
}

// detachStaleOutputBuffers removes buffers for panes that no longer exist and
// returns them for lock-free stopping.
func (a *App) detachStaleOutputBuffers(existingPanes map[string]struct{}) map[string]*terminal.OutputBuffer {
	a.outputMu.Lock()
	if len(a.outputBuffers) == 0 {
		a.outputMu.Unlock()
		return nil
	}
	detached := make(map[string]*terminal.OutputBuffer)
	for paneID, buffer := range a.outputBuffers {
		if _, ok := existingPanes[paneID]; ok {
			continue
		}
		detached[paneID] = buffer
		delete(a.outputBuffers, paneID)
	}
	a.outputMu.Unlock()
	return detached
}

// stopDetachedOutputBuffers stops detached buffers outside outputMu and removes
// corresponding pane state entries.
func (a *App) stopDetachedOutputBuffers(buffers map[string]*terminal.OutputBuffer) {
	for paneID, buffer := range buffers {
		if buffer != nil {
			buffer.Stop()
		}
		if a.paneStates != nil {
			a.paneStates.RemovePane(paneID)
		}
	}
}

func (a *App) stopOutputBuffer(paneID string) {
	a.outputMu.Lock()
	buffer := a.outputBuffers[paneID]
	delete(a.outputBuffers, paneID)
	a.outputMu.Unlock()

	if buffer != nil {
		buffer.Stop()
	}
	if a.paneStates != nil {
		a.paneStates.RemovePane(paneID)
	}
}

func (a *App) emitSnapshot() {
	// SessionManager pointer is initialized once at startup and kept stable.
	// During shutdown it is closed but not replaced, so a local snapshot is safe.
	sessions := a.sessions
	if a.runtimeContext() == nil || sessions == nil {
		return
	}
	snapshots := sessions.Snapshot()
	a.syncPaneStates(snapshots)
	delta, changed, initial := a.snapshotDelta(snapshots)
	if initial {
		a.emitRuntimeEvent("tmux:snapshot", snapshots)
		a.recordSnapshotEmission("full", payloadSizeBytes(snapshots))
		return
	}
	if !changed {
		return
	}
	a.emitRuntimeEvent("tmux:snapshot-delta", delta)
	a.recordSnapshotEmission("delta", payloadSizeBytes(delta))
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
