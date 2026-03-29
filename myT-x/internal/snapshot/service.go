package snapshot

// service.go — Service struct definition, constructor, and lifecycle management.
//
// The snapshot pipeline is split across multiple files:
//
//	service.go   — Deps, Service struct, NewService, Shutdown (this file)
//	output.go    — Pane output buffering, flush management, pane feed worker
//	cache.go     — Snapshot cache, topology sync, debounced emission
//	delta.go     — Snapshot equality comparison and delta computation
//	metrics.go   — Payload size estimation and emission metrics recording
//	feed.go      — feedBytePool and paneFeedItem (zero-alloc PTY chunk path)
//	convert.go   — Payload type conversion helpers
//	policy.go    — Event-to-snapshot emission policy map

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"myT-x/internal/apptypes"
	"myT-x/internal/terminal"
	"myT-x/internal/tmux"
	"myT-x/internal/workerutil"
)

// Deps provides external dependencies for the snapshot pipeline Service.
// All function fields are closures that close over the App or its sub-objects,
// following the project convention for dependency injection.
type Deps struct {
	// RuntimeContext provides the Wails runtime context.
	// Returns nil before startup completes or after shutdown begins.
	RuntimeContext func() context.Context

	// Emitter sends events to the frontend.
	Emitter apptypes.RuntimeEventEmitter

	// SessionsReady returns true if the session manager is initialized.
	SessionsReady func() bool

	// SessionSnapshot returns the current session snapshots.
	// Must only be called when SessionsReady() returns true.
	SessionSnapshot func() []tmux.SessionSnapshot

	// TopologyGeneration returns the current topology generation counter.
	// Must only be called when SessionsReady() returns true.
	TopologyGeneration func() uint64

	// UpdateActivityByPaneID updates the activity timestamp for the given pane.
	// Returns true if the pane exists and its idle state changed.
	// May be nil; nil is treated as no-op (always returns false).
	UpdateActivityByPaneID func(paneID string) bool

	// DeliverPaneOutput delivers flushed pane output to the frontend.
	// The implementation chooses between WebSocket and IPC based on connection state.
	DeliverPaneOutput func(ctx context.Context, paneID string, data []byte)

	// Pane state management closures (close over *panestate.Manager).
	// All may be nil; nil is defaulted to no-op in NewService.
	PaneStateFeedTrimmed func(paneID string, chunk []byte)
	PaneStateEnsurePane  func(paneID string, width, height int)
	PaneStateSetActive   func(active map[string]struct{})
	PaneStateRetainPanes func(alive map[string]struct{})
	PaneStateRemovePane  func(paneID string)
	HasPaneStates        func() bool

	// LaunchWorker starts a background worker goroutine with panic recovery.
	LaunchWorker func(name string, ctx context.Context, fn func(ctx context.Context), opts workerutil.RecoveryOptions)

	// BaseRecoveryOptions returns the default recovery options for worker goroutines.
	BaseRecoveryOptions func() workerutil.RecoveryOptions
}

// Service handles the snapshot pipeline: pane output buffering, debounced
// snapshot emission, delta computation, and metrics.
//
// Thread-safety:
//   - outputMu protects outputFlusher and paneFeedStop.
//   - snapshotMu protects snapshotCache, snapshotPrimed, snapshotLastTopology.
//   - snapshotDeltaMu serializes concurrent delta computation paths.
//   - snapshotRequestMu protects the debounce state.
//   - snapshotMetricsMu protects snapshotStats.
//
// Lock ordering (outer -> inner):
//
//	snapshotDeltaMu -> snapshotMu (snapshotDelta acquires snapshotMu while holding snapshotDeltaMu)
//
// Independent locks: outputMu, snapshotRequestMu, snapshotMetricsMu.
type Service struct {
	deps           Deps
	shutdownCalled atomic.Bool // set true at the start of Shutdown; public methods return early.

	// Output buffering.
	outputMu      sync.Mutex
	outputFlusher *terminal.OutputFlushManager
	paneFeedCh    chan paneFeedItem
	paneFeedStop  context.CancelFunc // protected by outputMu

	// Snapshot cache.
	snapshotMu           sync.Mutex
	snapshotDeltaMu      sync.Mutex
	snapshotCache        map[string]tmux.SessionSnapshot
	snapshotPrimed       bool
	snapshotLastTopology uint64

	// Snapshot request debounce.
	snapshotRequestMu         sync.Mutex
	snapshotRequestTimer      *time.Timer
	snapshotRequestGeneration uint64
	snapshotRequestDispatched uint64

	// Metrics.
	snapshotMetricsMu sync.Mutex
	snapshotStats     snapshotMetrics
}

// NewService creates a snapshot pipeline service.
// Required deps: RuntimeContext, Emitter, SessionsReady, SessionSnapshot,
// TopologyGeneration, DeliverPaneOutput, LaunchWorker, BaseRecoveryOptions.
// Optional deps (nil → no-op): UpdateActivityByPaneID, PaneState* closures, HasPaneStates.
func NewService(deps Deps) *Service {
	if deps.RuntimeContext == nil {
		panic("snapshot.NewService: RuntimeContext must not be nil")
	}
	if deps.Emitter == nil {
		panic("snapshot.NewService: Emitter must not be nil")
	}
	if deps.SessionsReady == nil {
		panic("snapshot.NewService: SessionsReady must not be nil")
	}
	if deps.SessionSnapshot == nil {
		panic("snapshot.NewService: SessionSnapshot must not be nil")
	}
	if deps.TopologyGeneration == nil {
		panic("snapshot.NewService: TopologyGeneration must not be nil")
	}
	if deps.DeliverPaneOutput == nil {
		panic("snapshot.NewService: DeliverPaneOutput must not be nil")
	}
	if deps.LaunchWorker == nil {
		panic("snapshot.NewService: LaunchWorker must not be nil")
	}
	if deps.BaseRecoveryOptions == nil {
		panic("snapshot.NewService: BaseRecoveryOptions must not be nil")
	}

	// Default nil-safe optional deps.
	if deps.UpdateActivityByPaneID == nil {
		deps.UpdateActivityByPaneID = func(string) bool { return false }
	}
	if deps.HasPaneStates == nil {
		deps.HasPaneStates = func() bool { return false }
	}
	if deps.PaneStateFeedTrimmed == nil {
		deps.PaneStateFeedTrimmed = func(string, []byte) {}
	}
	if deps.PaneStateEnsurePane == nil {
		deps.PaneStateEnsurePane = func(string, int, int) {}
	}
	if deps.PaneStateSetActive == nil {
		deps.PaneStateSetActive = func(map[string]struct{}) {}
	}
	if deps.PaneStateRetainPanes == nil {
		deps.PaneStateRetainPanes = func(map[string]struct{}) {}
	}
	if deps.PaneStateRemovePane == nil {
		deps.PaneStateRemovePane = func(string) {}
	}

	return &Service{
		deps:          deps,
		paneFeedCh:    make(chan paneFeedItem, paneFeedChSize),
		snapshotCache: map[string]tmux.SessionSnapshot{},
	}
}

// Shutdown stops the pane feed worker, clears the snapshot request timer,
// detaches all output buffers, cleans up pane states, and resets internal caches.
// Returns the list of pane IDs that were detached (for external cleanup).
func (s *Service) Shutdown() []string {
	s.shutdownCalled.Store(true)

	s.StopPaneFeedWorker()
	s.ClearSnapshotRequestTimer()

	s.snapshotRequestMu.Lock()
	s.snapshotRequestGeneration = 0
	s.snapshotRequestDispatched = 0
	s.snapshotRequestMu.Unlock()

	removed := s.DetachAllOutputBuffers()
	s.CleanupDetachedPaneStates(removed)

	s.snapshotMu.Lock()
	s.snapshotCache = map[string]tmux.SessionSnapshot{}
	s.snapshotPrimed = false
	s.snapshotLastTopology = 0
	s.snapshotMu.Unlock()

	s.snapshotMetricsMu.Lock()
	s.snapshotStats = snapshotMetrics{}
	s.snapshotMetricsMu.Unlock()

	return removed
}
