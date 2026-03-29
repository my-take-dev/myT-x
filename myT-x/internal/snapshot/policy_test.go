package snapshot

import (
	"testing"
)

// ---------------------------------------------------------------------------
// ShouldEmitSnapshotForEvent
// ---------------------------------------------------------------------------

func TestShouldEmitSnapshotForEvent(t *testing.T) {
	tests := []struct {
		event string
		want  bool
	}{
		// Registered events with trigger=true.
		{"tmux:session-created", true},
		{"tmux:session-destroyed", true},
		{"tmux:session-emptied", true},
		{"tmux:session-renamed", true},
		{"tmux:pane-created", true},
		{"tmux:layout-changed", true},
		{"tmux:pane-focused", true},
		{"tmux:pane-renamed", true},
		{"tmux:window-created", true},
		{"tmux:window-destroyed", true},
		{"tmux:window-renamed", true},

		// Unregistered events.
		{"tmux:pane-output", false},
		{"tmux:unknown-event", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			got := ShouldEmitSnapshotForEvent(tt.event)
			if got != tt.want {
				t.Errorf("ShouldEmitSnapshotForEvent(%q) = %v, want %v", tt.event, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ShouldBypassSnapshotDebounceForEvent
// ---------------------------------------------------------------------------

func TestShouldBypassSnapshotDebounceForEvent(t *testing.T) {
	tests := []struct {
		event string
		want  bool
	}{
		// Events that bypass debounce (immediate emission).
		{"tmux:session-created", true},
		{"tmux:session-destroyed", true},
		{"tmux:session-emptied", true},
		{"tmux:session-renamed", true},
		{"tmux:pane-focused", true},
		{"tmux:pane-renamed", true},
		{"tmux:window-created", true},
		{"tmux:window-destroyed", true},
		{"tmux:window-renamed", true},

		// Events that do NOT bypass debounce.
		{"tmux:pane-created", false},
		{"tmux:layout-changed", false},

		// Unregistered events.
		{"tmux:pane-output", false},
		{"tmux:unknown-event", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			got := ShouldBypassSnapshotDebounceForEvent(tt.event)
			if got != tt.want {
				t.Errorf("ShouldBypassSnapshotDebounceForEvent(%q) = %v, want %v", tt.event, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Consistency: all policies with trigger=true must have explicit bypass tests
// ---------------------------------------------------------------------------

func TestSnapshotEventPolicyConsistency(t *testing.T) {
	for event, policy := range snapshotEventPolicies {
		t.Run(event, func(t *testing.T) {
			// trigger implies the event should be registered.
			if policy.trigger && !ShouldEmitSnapshotForEvent(event) {
				t.Errorf("policy trigger=true but ShouldEmitSnapshotForEvent(%q) returned false", event)
			}

			// bypassDebounce must imply trigger.
			if policy.bypassDebounce && !policy.trigger {
				t.Errorf("policy bypassDebounce=true but trigger=false for %q; bypass without trigger is invalid", event)
			}

			// ShouldBypass must match policy.
			gotBypass := ShouldBypassSnapshotDebounceForEvent(event)
			if gotBypass != policy.bypassDebounce {
				t.Errorf("ShouldBypassSnapshotDebounceForEvent(%q) = %v, want %v", event, gotBypass, policy.bypassDebounce)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Guard: every registered event must have an explicit test case above
// ---------------------------------------------------------------------------

func TestAllRegisteredEventsHaveSnapshotPolicyTests(t *testing.T) {
	// This set mirrors the test table in TestShouldEmitSnapshotForEvent above.
	// If a new event is added to snapshotEventPolicies, this test fails.
	testedEvents := map[string]bool{
		"tmux:session-created":   true,
		"tmux:session-destroyed": true,
		"tmux:session-emptied":   true,
		"tmux:session-renamed":   true,
		"tmux:pane-created":      true,
		"tmux:layout-changed":    true,
		"tmux:pane-focused":      true,
		"tmux:pane-renamed":      true,
		"tmux:window-created":    true,
		"tmux:window-destroyed":  true,
		"tmux:window-renamed":    true,
	}

	for event := range snapshotEventPolicies {
		if !testedEvents[event] {
			t.Errorf("event %q is registered in snapshotEventPolicies but has no explicit test case; add it to TestShouldEmitSnapshotForEvent and TestShouldBypassSnapshotDebounceForEvent", event)
		}
	}
}
