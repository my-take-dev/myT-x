package snapshot

import (
	"context"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"myT-x/internal/apptypes"
	"myT-x/internal/panestate"
	"myT-x/internal/tmux"
	"myT-x/internal/workerutil"
)

// ---------------------------------------------------------------------------
// Shared test helpers (used by service_test.go, output_test.go, cache_test.go)
// ---------------------------------------------------------------------------

// newTestService creates a Service with minimal, test-friendly dependencies.
// The returned snapshot list and topology generation are configurable via the
// captured variables; callers mutate them before triggering service methods.
func newTestService(t *testing.T) *Service {
	t.Helper()

	var (
		snapshots          []tmux.SessionSnapshot
		topologyGeneration uint64 = 1
		bgWG               sync.WaitGroup
	)

	psm := panestate.NewManager(4096)

	svc := NewService(Deps{
		RuntimeContext: func() context.Context { return context.Background() },
		Emitter:        apptypes.NoopEmitter{},
		SessionsReady:  func() bool { return true },
		SessionSnapshot: func() []tmux.SessionSnapshot {
			return snapshots
		},
		TopologyGeneration: func() uint64 { return topologyGeneration },
		DeliverPaneOutput: func(_ context.Context, _ string, _ []byte) {
			// no-op capture; tests that need to assert delivery override via Deps.
		},
		PaneStateFeedTrimmed: psm.FeedTrimmed,
		PaneStateEnsurePane:  psm.EnsurePane,
		PaneStateSetActive:   psm.SetActivePanes,
		PaneStateRetainPanes: psm.RetainPanes,
		PaneStateRemovePane:  psm.RemovePane,
		HasPaneStates:        func() bool { return true },
		LaunchWorker: func(name string, ctx context.Context, fn func(ctx context.Context), opts workerutil.RecoveryOptions) {
			workerutil.RunWithPanicRecovery(ctx, name, &bgWG, fn, opts)
		},
		BaseRecoveryOptions: func() workerutil.RecoveryOptions {
			return workerutil.RecoveryOptions{MaxRetries: 1}
		},
	})
	t.Cleanup(func() {
		svc.Shutdown()
		bgWG.Wait()
	})
	return svc
}

// waitForCondition polls condFn every 5 ms until it returns true or timeout
// elapses. On timeout the test is failed with msg.
func waitForCondition(t *testing.T, timeout time.Duration, condFn func() bool, msg string) {
	t.Helper()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		select {
		case <-ticker.C:
			if condFn() {
				return
			}
		case <-deadline.C:
			t.Fatalf("waitForCondition timed out after %v: %s", timeout, msg)
		}
	}
}

// validDeps returns a Deps with all required fields populated.
func validDeps() Deps {
	return Deps{
		RuntimeContext:     func() context.Context { return context.Background() },
		Emitter:            apptypes.NoopEmitter{},
		SessionsReady:      func() bool { return true },
		SessionSnapshot:    func() []tmux.SessionSnapshot { return nil },
		TopologyGeneration: func() uint64 { return 0 },
		DeliverPaneOutput:  func(context.Context, string, []byte) {},
		LaunchWorker: func(_ string, _ context.Context, _ func(context.Context), _ workerutil.RecoveryOptions) {
		},
		BaseRecoveryOptions: func() workerutil.RecoveryOptions {
			return workerutil.RecoveryOptions{MaxRetries: 1}
		},
	}
}

// ---------------------------------------------------------------------------
// Test-only emitter helpers (used by cache_test.go and service_test.go)
// ---------------------------------------------------------------------------

type emittedEvent struct {
	name    string
	payload any
}

type recordingEmitter struct {
	mu   sync.Mutex
	evts []emittedEvent
}

func (e *recordingEmitter) Emit(name string, payload any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.evts = append(e.evts, emittedEvent{name: name, payload: payload})
}

func (e *recordingEmitter) EmitWithContext(_ context.Context, name string, payload any) {
	e.Emit(name, payload)
}

func (e *recordingEmitter) events() []emittedEvent {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]emittedEvent, len(e.evts))
	copy(out, e.evts)
	return out
}

type countingEmitter struct {
	count *atomic.Int32
}

func (e *countingEmitter) Emit(string, any) {
	e.count.Add(1)
}

func (e *countingEmitter) EmitWithContext(_ context.Context, _ string, _ any) {
	e.count.Add(1)
}

// ---------------------------------------------------------------------------
// NewService validation
// ---------------------------------------------------------------------------

func TestNewServicePanicsOnNilRequiredDeps(t *testing.T) {
	requiredDeps := []struct {
		name    string
		modify  func(d *Deps)
		wantMsg string
	}{
		{
			name:    "RuntimeContext",
			modify:  func(d *Deps) { d.RuntimeContext = nil },
			wantMsg: "RuntimeContext must not be nil",
		},
		{
			name:    "Emitter",
			modify:  func(d *Deps) { d.Emitter = nil },
			wantMsg: "Emitter must not be nil",
		},
		{
			name:    "SessionsReady",
			modify:  func(d *Deps) { d.SessionsReady = nil },
			wantMsg: "SessionsReady must not be nil",
		},
		{
			name:    "SessionSnapshot",
			modify:  func(d *Deps) { d.SessionSnapshot = nil },
			wantMsg: "SessionSnapshot must not be nil",
		},
		{
			name:    "TopologyGeneration",
			modify:  func(d *Deps) { d.TopologyGeneration = nil },
			wantMsg: "TopologyGeneration must not be nil",
		},
		{
			name:    "DeliverPaneOutput",
			modify:  func(d *Deps) { d.DeliverPaneOutput = nil },
			wantMsg: "DeliverPaneOutput must not be nil",
		},
		{
			name:    "LaunchWorker",
			modify:  func(d *Deps) { d.LaunchWorker = nil },
			wantMsg: "LaunchWorker must not be nil",
		},
		{
			name:    "BaseRecoveryOptions",
			modify:  func(d *Deps) { d.BaseRecoveryOptions = nil },
			wantMsg: "BaseRecoveryOptions must not be nil",
		},
	}

	for _, tt := range requiredDeps {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("expected panic for nil %s dep, got none", tt.name)
				}
				msg, ok := r.(string)
				if !ok {
					t.Fatalf("panic value is not string: %T", r)
				}
				if got := msg; got != "snapshot.NewService: "+tt.wantMsg {
					t.Errorf("panic message = %q, want suffix %q", got, tt.wantMsg)
				}
			}()

			d := validDeps()
			tt.modify(&d)
			NewService(d)
		})
	}
}

func TestNewServiceDefaultsOptionalDeps(t *testing.T) {
	d := validDeps()
	// Leave optional deps nil.
	d.UpdateActivityByPaneID = nil
	d.HasPaneStates = nil
	d.PaneStateFeedTrimmed = nil
	d.PaneStateEnsurePane = nil
	d.PaneStateSetActive = nil
	d.PaneStateRetainPanes = nil
	d.PaneStateRemovePane = nil

	svc := NewService(d)
	if svc == nil {
		t.Fatal("NewService returned nil with nil optional deps")
	}
	// UpdateActivityByPaneID must default to a no-op that returns false.
	if svc.deps.UpdateActivityByPaneID == nil {
		t.Error("UpdateActivityByPaneID was not defaulted")
	}
	if svc.deps.UpdateActivityByPaneID("any") {
		t.Error("default UpdateActivityByPaneID should return false")
	}
	// HasPaneStates must default to a no-op that returns false.
	if svc.deps.HasPaneStates == nil {
		t.Error("HasPaneStates was not defaulted")
	}
	if svc.deps.HasPaneStates() {
		t.Error("default HasPaneStates should return false")
	}
	// All PaneState closures must be defaulted to non-nil no-ops.
	if svc.deps.PaneStateFeedTrimmed == nil {
		t.Error("PaneStateFeedTrimmed was not defaulted")
	}
	if svc.deps.PaneStateEnsurePane == nil {
		t.Error("PaneStateEnsurePane was not defaulted")
	}
	if svc.deps.PaneStateSetActive == nil {
		t.Error("PaneStateSetActive was not defaulted")
	}
	if svc.deps.PaneStateRetainPanes == nil {
		t.Error("PaneStateRetainPanes was not defaulted")
	}
	if svc.deps.PaneStateRemovePane == nil {
		t.Error("PaneStateRemovePane was not defaulted")
	}
	// Verify no-op defaults don't panic.
	svc.deps.PaneStateFeedTrimmed("%0", []byte("test"))
	svc.deps.PaneStateEnsurePane("%0", 80, 24)
	svc.deps.PaneStateSetActive(map[string]struct{}{"%0": {}})
	svc.deps.PaneStateRetainPanes(map[string]struct{}{"%0": {}})
	svc.deps.PaneStateRemovePane("%0")
}

// ---------------------------------------------------------------------------
// Shutdown
// ---------------------------------------------------------------------------

func TestShutdown(t *testing.T) {
	svc := newTestService(t)

	// Prime snapshot cache.
	svc.emitSnapshot()

	removed := svc.Shutdown()
	// After shutdown the snapshot cache must be reset.
	svc.snapshotMu.Lock()
	cachePrimed := svc.snapshotPrimed
	cacheLen := len(svc.snapshotCache)
	svc.snapshotMu.Unlock()

	if cachePrimed {
		t.Error("snapshotPrimed should be false after shutdown")
	}
	if cacheLen != 0 {
		t.Errorf("snapshotCache should be empty after shutdown, got %d entries", cacheLen)
	}

	// Double shutdown must not panic.
	removed2 := svc.Shutdown()
	_ = removed
	_ = removed2
}

// ---------------------------------------------------------------------------
// Shutdown circuit breaker (I-8)
// ---------------------------------------------------------------------------

func TestShutdownCircuitBreakerStopsPublicMethods(t *testing.T) {
	var emitCount atomic.Int32

	d := validDeps()
	d.SessionSnapshot = func() []tmux.SessionSnapshot { return nil }
	d.Emitter = &countingEmitter{count: &emitCount}
	svc := NewService(d)

	svc.Shutdown()

	// After shutdown, public methods must return early without effect.
	svc.HandlePaneOutputEvent(&tmux.PaneOutputEvent{PaneID: "%0", Data: []byte("data")})
	svc.RequestSnapshot(true)
	svc.RequestSnapshot(false)

	// No emissions should have occurred after shutdown.
	if emitCount.Load() != 0 {
		t.Errorf("expected 0 emissions after shutdown, got %d", emitCount.Load())
	}
}

// ---------------------------------------------------------------------------
// Field count guard: Deps struct
// ---------------------------------------------------------------------------

func TestDepsFieldCount(t *testing.T) {
	// Deps has 15 fields. If a field is added or removed, this test fails,
	// reminding the author to update newTestService and validDeps helpers.
	const wantFields = 15
	got := reflect.TypeFor[Deps]().NumField()
	if got != wantFields {
		t.Errorf("Deps has %d fields, want %d; update test helpers when fields change", got, wantFields)
	}
}
