package snapshot

// policy.go — Event-to-snapshot emission policy map.

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
// DESIGN: All current entries have trigger=true. ShouldEmitSnapshotForEvent checks
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
	"tmux:session-emptied":   {trigger: true, bypassDebounce: true},
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

// ShouldEmitSnapshotForEvent returns whether the named backend event should
// trigger a snapshot emission.
func ShouldEmitSnapshotForEvent(name string) bool {
	policy, ok := snapshotEventPolicies[name]
	return ok && policy.trigger
}

// ShouldBypassSnapshotDebounceForEvent returns whether the named backend event
// should bypass the snapshot coalesce debounce window.
func ShouldBypassSnapshotDebounceForEvent(name string) bool {
	policy, ok := snapshotEventPolicies[name]
	return ok && policy.bypassDebounce
}

// SnapshotPolicyForEvent returns whether the event should trigger a snapshot
// and whether it should bypass the debounce window, in a single map lookup.
func SnapshotPolicyForEvent(name string) (shouldEmit bool, bypassDebounce bool) {
	p, ok := snapshotEventPolicies[name]
	if !ok {
		return false, false
	}
	return p.trigger, p.bypassDebounce
}
