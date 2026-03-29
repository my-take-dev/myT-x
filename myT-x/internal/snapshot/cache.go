package snapshot

// cache.go — Snapshot cache, topology synchronization, and debounced emission.
//
// Methods in this file manage the snapshot pipeline:
//   - emitSnapshot collects and emits full/delta snapshots to the frontend.
//   - shouldSyncPaneStates / syncPaneStates reconcile pane state with topology.
//   - RequestSnapshot provides a debounced entry point for snapshot emission.
//   - ClearSnapshotRequestTimer cleans up the debounce timer.

import (
	"log/slog"
	"time"

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
)

// emitSnapshot collects and emits a snapshot or delta to the frontend.
func (s *Service) emitSnapshot() {
	if s.shutdownCalled.Load() {
		return
	}
	ctx := s.deps.RuntimeContext()
	if ctx == nil {
		slog.Debug("[snapshot] skip emitSnapshot: runtime context is nil")
		return
	}
	if !s.deps.SessionsReady() {
		slog.Debug("[snapshot] skip emitSnapshot: sessions manager is nil")
		return
	}
	snapshots := s.deps.SessionSnapshot()
	if s.shouldSyncPaneStates(s.deps.TopologyGeneration()) {
		s.syncPaneStates(snapshots)
	}
	delta, changed, initial := s.snapshotDelta(snapshots)
	if initial {
		s.deps.Emitter.EmitWithContext(ctx, "tmux:snapshot", snapshots)
		s.recordSnapshotEmission("full", snapshots)
		return
	}
	if !changed {
		return
	}
	s.deps.Emitter.EmitWithContext(ctx, "tmux:snapshot-delta", delta)
	s.recordSnapshotEmission("delta", delta)
}

// shouldSyncPaneStates tracks the last synced topology generation and returns
// whether pane state synchronization should run for this snapshot.
// NOTE: This predicate has side effects by updating snapshotLastTopology.
func (s *Service) shouldSyncPaneStates(topologyGeneration uint64) bool {
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	if s.snapshotLastTopology == topologyGeneration {
		return false
	}
	s.snapshotLastTopology = topologyGeneration
	return true
}

func (s *Service) syncPaneStates(snapshots []tmux.SessionSnapshot) {
	if !s.deps.HasPaneStates() {
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
				// PaneStateEnsurePane is guaranteed non-nil (defaulted in NewService).
				s.deps.PaneStateEnsurePane(pane.ID, pane.Width, pane.Height)
			}
		}
	}
	// PaneStateSetActive and PaneStateRetainPanes are guaranteed non-nil (defaulted in NewService).
	s.deps.PaneStateSetActive(active)
	s.deps.PaneStateRetainPanes(alive)
}

// RequestSnapshot triggers a snapshot emission with leading-edge debounce.
//
// immediate=true (structural events: session/pane/window created/destroyed):
//
//	Cancels any pending timer and emits synchronously on the caller's goroutine.
//	The generation counter prevents duplicate emissions when multiple callers
//	race into the immediate path simultaneously.
//
// immediate=false (output-driven activity updates):
//
//	Arms a one-shot timer for snapshotCoalesceWindow. If another request
//	arrives before the timer fires, the timer is left running (no reset);
//	this bounds the worst-case delay to one coalesce window regardless of
//	how many events arrive.
func (s *Service) RequestSnapshot(immediate bool) {
	if s.shutdownCalled.Load() {
		return
	}
	if s.deps.RuntimeContext() == nil {
		slog.Debug("[snapshot] skip RequestSnapshot: runtime context is nil")
		return
	}
	if !s.deps.SessionsReady() {
		slog.Debug("[snapshot] skip RequestSnapshot: sessions manager is nil")
		return
	}

	emitNow := false
	s.snapshotRequestMu.Lock()
	s.snapshotRequestGeneration++
	currentGeneration := s.snapshotRequestGeneration

	if immediate {
		if s.snapshotRequestTimer != nil {
			s.snapshotRequestTimer.Stop()
			s.snapshotRequestTimer = nil
		}
		if s.snapshotRequestDispatched < currentGeneration {
			s.snapshotRequestDispatched = currentGeneration
			emitNow = true
		}
		s.snapshotRequestMu.Unlock()
		if emitNow {
			s.emitSnapshot()
		}
		return
	}

	if s.snapshotRequestTimer == nil {
		s.snapshotRequestTimer = time.AfterFunc(snapshotCoalesceWindow, s.flushSnapshotRequest)
	}
	s.snapshotRequestMu.Unlock()
}

func (s *Service) flushSnapshotRequest() {
	ctx := s.deps.RuntimeContext()
	sessionsReady := s.deps.SessionsReady()
	emitNow := false

	s.snapshotRequestMu.Lock()
	s.snapshotRequestTimer = nil
	if ctx == nil {
		s.snapshotRequestMu.Unlock()
		slog.Debug("[snapshot] skip flushSnapshotRequest: runtime context is nil")
		return
	}
	if !sessionsReady {
		s.snapshotRequestMu.Unlock()
		slog.Debug("[snapshot] skip flushSnapshotRequest: sessions manager is nil")
		return
	}
	if s.snapshotRequestDispatched < s.snapshotRequestGeneration {
		s.snapshotRequestDispatched = s.snapshotRequestGeneration
		emitNow = true
	}
	s.snapshotRequestMu.Unlock()

	if emitNow {
		s.emitSnapshot()
	}
}

// ClearSnapshotRequestTimer stops and nils any pending snapshot request timer.
func (s *Service) ClearSnapshotRequestTimer() {
	s.snapshotRequestMu.Lock()
	if s.snapshotRequestTimer != nil {
		s.snapshotRequestTimer.Stop()
		s.snapshotRequestTimer = nil
	}
	s.snapshotRequestMu.Unlock()
}
