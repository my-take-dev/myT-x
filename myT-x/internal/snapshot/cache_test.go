package snapshot

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"myT-x/internal/tmux"
)

// ---------------------------------------------------------------------------
// RequestSnapshot
// ---------------------------------------------------------------------------

func TestRequestSnapshotNilGuards(t *testing.T) {
	t.Run("nil_context", func(t *testing.T) {
		d := validDeps()
		d.RuntimeContext = func() context.Context { return nil }
		svc := NewService(d)
		// Must not panic.
		svc.RequestSnapshot(false)
		svc.RequestSnapshot(true)
	})

	t.Run("sessions_not_ready", func(t *testing.T) {
		d := validDeps()
		d.SessionsReady = func() bool { return false }
		svc := NewService(d)
		// Must not panic.
		svc.RequestSnapshot(false)
		svc.RequestSnapshot(true)
	})
}

func TestRequestSnapshotCoalescesBurst(t *testing.T) {
	var emitCount atomic.Int32
	var snapshots []tmux.SessionSnapshot

	d := validDeps()
	d.SessionSnapshot = func() []tmux.SessionSnapshot { return snapshots }
	d.Emitter = &countingEmitter{count: &emitCount}
	svc := NewService(d)
	t.Cleanup(func() { svc.Shutdown() })

	// Fire 10 non-immediate requests in rapid succession.
	for range 10 {
		svc.RequestSnapshot(false)
	}

	// Wait for the coalesce window to fire.
	waitForCondition(t, 500*time.Millisecond, func() bool {
		return emitCount.Load() > 0
	}, "coalesced snapshot should have fired")

	// At most 2 emissions (first priming + 1 coalesced).
	got := emitCount.Load()
	if got > 2 {
		t.Errorf("expected at most 2 emissions, got %d", got)
	}
}

func TestRequestSnapshotImmediateCancelsPendingDebounce(t *testing.T) {
	var emitCount atomic.Int32
	var snapshots []tmux.SessionSnapshot

	d := validDeps()
	d.SessionSnapshot = func() []tmux.SessionSnapshot { return snapshots }
	d.Emitter = &countingEmitter{count: &emitCount}
	svc := NewService(d)
	t.Cleanup(func() { svc.Shutdown() })

	// Arm a debounced request then immediately fire an immediate one.
	svc.RequestSnapshot(false)
	svc.RequestSnapshot(true)

	// The immediate emission should have happened synchronously.
	if emitCount.Load() < 1 {
		t.Error("immediate RequestSnapshot should have emitted at least once")
	}
}

// ---------------------------------------------------------------------------
// ClearSnapshotRequestTimer
// ---------------------------------------------------------------------------

func TestClearSnapshotRequestTimerStopsScheduledCallback(t *testing.T) {
	var emitCount atomic.Int32

	d := validDeps()
	d.SessionSnapshot = func() []tmux.SessionSnapshot { return nil }
	d.Emitter = &countingEmitter{count: &emitCount}
	svc := NewService(d)
	t.Cleanup(func() { svc.Shutdown() })

	svc.RequestSnapshot(false)
	svc.ClearSnapshotRequestTimer()

	// Wait longer than the coalesce window; emission should NOT fire.
	time.Sleep(120 * time.Millisecond)
	if emitCount.Load() != 0 {
		t.Errorf("expected 0 emissions after clearing timer, got %d", emitCount.Load())
	}
}

// ---------------------------------------------------------------------------
// EmitSnapshot
// ---------------------------------------------------------------------------

func TestEmitSnapshotFirstCallEmitsFullSnapshot(t *testing.T) {
	rec := &recordingEmitter{}

	d := validDeps()
	d.Emitter = rec
	d.SessionSnapshot = func() []tmux.SessionSnapshot {
		return []tmux.SessionSnapshot{
			{Name: "s1", ID: 1},
		}
	}
	svc := NewService(d)
	t.Cleanup(func() { svc.Shutdown() })

	svc.emitSnapshot()

	if len(rec.events()) == 0 {
		t.Fatal("expected at least one emission")
	}
	first := rec.events()[0]
	if first.name != "tmux:snapshot" {
		t.Errorf("first emission event = %q, want %q", first.name, "tmux:snapshot")
	}
}

func TestEmitSnapshotSecondCallWithNoChangeEmitsNothing(t *testing.T) {
	rec := &recordingEmitter{}

	d := validDeps()
	d.Emitter = rec
	d.SessionSnapshot = func() []tmux.SessionSnapshot {
		return []tmux.SessionSnapshot{{Name: "s1", ID: 1}}
	}
	svc := NewService(d)
	t.Cleanup(func() { svc.Shutdown() })

	svc.emitSnapshot() // priming (full)
	svc.emitSnapshot() // no change

	evts := rec.events()
	if len(evts) != 1 {
		t.Fatalf("expected exactly 1 emission (full priming), got %d", len(evts))
	}
}

func TestEmitSnapshotEmitsDeltaAfterChange(t *testing.T) {
	rec := &recordingEmitter{}
	currentSnapshots := []tmux.SessionSnapshot{{Name: "s1", ID: 1}}

	d := validDeps()
	d.Emitter = rec
	d.SessionSnapshot = func() []tmux.SessionSnapshot {
		return currentSnapshots
	}
	svc := NewService(d)
	t.Cleanup(func() { svc.Shutdown() })

	svc.emitSnapshot() // priming

	// Mutate: add a new session.
	currentSnapshots = []tmux.SessionSnapshot{
		{Name: "s1", ID: 1},
		{Name: "s2", ID: 2},
	}
	svc.emitSnapshot()

	evts := rec.events()
	if len(evts) < 2 {
		t.Fatalf("expected at least 2 emissions, got %d", len(evts))
	}
	second := evts[1]
	if second.name != "tmux:snapshot-delta" {
		t.Errorf("second emission event = %q, want %q", second.name, "tmux:snapshot-delta")
	}
}

// ---------------------------------------------------------------------------
// shouldSyncPaneStates
// ---------------------------------------------------------------------------

func TestShouldSyncPaneStates(t *testing.T) {
	svc := newTestService(t)

	if !svc.shouldSyncPaneStates(1) {
		t.Error("first call with new generation should return true")
	}
	if svc.shouldSyncPaneStates(1) {
		t.Error("same generation should return false")
	}
	if !svc.shouldSyncPaneStates(2) {
		t.Error("bumped generation should return true")
	}
}

// ---------------------------------------------------------------------------
// syncPaneStates (integration)
// ---------------------------------------------------------------------------

func TestSyncPaneStatesIntegration(t *testing.T) {
	var ensuredPanes sync.Map
	var retainedAlive map[string]struct{}
	var activeSet map[string]struct{}

	d := validDeps()
	d.HasPaneStates = func() bool { return true }
	d.PaneStateEnsurePane = func(paneID string, w, h int) {
		ensuredPanes.Store(paneID, [2]int{w, h})
	}
	d.PaneStateSetActive = func(active map[string]struct{}) {
		activeSet = active
	}
	d.PaneStateRetainPanes = func(alive map[string]struct{}) {
		retainedAlive = alive
	}
	svc := NewService(d)
	t.Cleanup(func() { svc.Shutdown() })

	snapshots := []tmux.SessionSnapshot{
		{
			Name: "s1",
			Windows: []tmux.WindowSnapshot{
				{
					Panes: []tmux.PaneSnapshot{
						{ID: "%0", Active: true, Width: 80, Height: 24},
						{ID: "%1", Active: false, Width: 80, Height: 24},
					},
				},
			},
		},
	}
	svc.syncPaneStates(snapshots)

	if _, ok := ensuredPanes.Load("%0"); !ok {
		t.Error("pane %0 was not ensured")
	}
	if _, ok := ensuredPanes.Load("%1"); !ok {
		t.Error("pane %1 was not ensured")
	}
	if _, ok := activeSet["%0"]; !ok {
		t.Error("pane %0 should be in active set")
	}
	if _, ok := activeSet["%1"]; ok {
		t.Error("pane %1 should NOT be in active set")
	}
	if _, ok := retainedAlive["%0"]; !ok {
		t.Error("pane %0 should be in alive set")
	}
}

// ---------------------------------------------------------------------------
// T-2: Shutdown timer callback race
// ---------------------------------------------------------------------------

func TestClearSnapshotRequestTimerRaceWithFlush(t *testing.T) {
	var emitCount atomic.Int32

	d := validDeps()
	d.SessionSnapshot = func() []tmux.SessionSnapshot { return nil }
	d.Emitter = &countingEmitter{count: &emitCount}
	svc := NewService(d)
	t.Cleanup(func() { svc.Shutdown() })

	// Arm debounced requests and immediately clear.
	// Repeated rapid arm+clear must not panic or corrupt state.
	for range 50 {
		svc.RequestSnapshot(false)
		svc.ClearSnapshotRequestTimer()
	}

	// Wait past the coalesce window to ensure no stale timers fire.
	time.Sleep(100 * time.Millisecond)

	// At most one emission could have squeezed through; no panics is the primary assertion.
	if emitCount.Load() > 1 {
		t.Errorf("expected at most 1 emission from race, got %d", emitCount.Load())
	}
}
