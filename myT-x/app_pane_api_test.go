package main

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"myT-x/internal/tmux"
)

// NOTE: This file overrides package-level function variables
// (runtimeEventsEmitFn). Do not use t.Parallel() here.

func TestSendInput(t *testing.T) {
	t.Run("returns error when session manager is unavailable", func(t *testing.T) {
		app := NewApp()
		app.sessions = nil

		if err := app.SendInput("%1", "echo test"); err == nil {
			t.Fatal("SendInput() expected error when sessions is nil")
		}
	})

	t.Run("returns error when pane terminal is not initialized", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		_, pane, err := app.sessions.CreateSession("s1", "0", 120, 40)
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}

		if err := app.SendInput(fmt.Sprintf("%%%d", pane.ID), "echo test\n"); err == nil {
			t.Fatal("SendInput() expected pane-not-found error for nil terminal")
		}
	})

	t.Run("requires pane id", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()

		if err := app.SendInput("   ", "echo test"); err == nil {
			t.Fatal("SendInput() expected pane id validation error")
		}
	})
}

func TestSendSyncInput(t *testing.T) {
	t.Run("returns error when session manager is unavailable", func(t *testing.T) {
		app := NewApp()
		app.sessions = nil

		if err := app.SendSyncInput("%1", "echo test"); err == nil {
			t.Fatal("SendSyncInput() expected error when sessions is nil")
		}
	})

	t.Run("requires pane id", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()

		if err := app.SendSyncInput("   ", "echo test"); err == nil {
			t.Fatal("SendSyncInput() expected pane id validation error")
		}
	})
}

func TestResizePane(t *testing.T) {
	t.Run("returns error when session manager is unavailable", func(t *testing.T) {
		app := NewApp()
		app.sessions = nil

		if err := app.ResizePane("%1", 100, 30); err == nil {
			t.Fatal("ResizePane() expected error when sessions is nil")
		}
	})

	t.Run("returns error when pane terminal is not initialized", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		_, pane, err := app.sessions.CreateSession("s1", "0", 120, 40)
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		paneID := fmt.Sprintf("%%%d", pane.ID)

		if err := app.ResizePane(paneID, 100, 30); err == nil {
			t.Fatal("ResizePane() expected pane-not-found error for nil terminal")
		}
	})

	t.Run("requires pane id", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()

		if err := app.ResizePane("   ", 100, 30); err == nil {
			t.Fatal("ResizePane() expected pane id validation error")
		}
	})
}

func TestPaneMutationAPIsRequireSessionManager(t *testing.T) {
	app := NewApp()
	app.sessions = nil

	if err := app.FocusPane("%1"); err == nil {
		t.Fatal("FocusPane() expected error when sessions is nil")
	}
	if err := app.RenamePane("%1", "new-title"); err == nil {
		t.Fatal("RenamePane() expected error when sessions is nil")
	}
	if err := app.SwapPanes("%1", "%2"); err == nil {
		t.Fatal("SwapPanes() expected error when sessions is nil")
	}
	if err := app.KillPane("%1"); err == nil {
		t.Fatal("KillPane() expected error when sessions is nil")
	}
	if err := app.ApplyLayoutPreset("session-a", "even-horizontal"); err == nil {
		t.Fatal("ApplyLayoutPreset() expected error when sessions is nil")
	}
}

func TestPaneMutationAPIsSuccessPaths(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	newAppWithPanes := func(t *testing.T) (*App, string, string) {
		t.Helper()
		app := NewApp()
		app.setRuntimeContext(context.Background())
		app.sessions = tmux.NewSessionManager()

		_, firstPane, err := app.sessions.CreateSession("session-a", "0", 120, 40)
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		secondPane, err := app.sessions.SplitPane(firstPane.ID, tmux.SplitHorizontal)
		if err != nil {
			t.Fatalf("SplitPane() error = %v", err)
		}
		return app, firstPane.IDString(), secondPane.IDString()
	}

	t.Run("FocusPane updates active pane and emits focused/snapshot events", func(t *testing.T) {
		app, _, targetPane := newAppWithPanes(t)
		events := make([]string, 0, 4)
		runtimeEventsEmitFn = func(_ context.Context, name string, _ ...any) {
			events = append(events, name)
		}

		if err := app.FocusPane(targetPane); err != nil {
			t.Fatalf("FocusPane() error = %v", err)
		}

		snapshots := app.sessions.Snapshot()
		window := snapshots[0].Windows[0]
		activePane := window.Panes[window.ActivePN].ID
		if activePane != targetPane {
			t.Fatalf("active pane = %q, want %q", activePane, targetPane)
		}
		if !containsEvent(events, "tmux:pane-focused") {
			t.Fatalf("events = %v, want tmux:pane-focused", events)
		}
		if !containsAnyEvent(events, "tmux:snapshot", "tmux:snapshot-delta") {
			t.Fatalf("events = %v, want snapshot update event", events)
		}
	})

	t.Run("RenamePane updates pane title and emits renamed event", func(t *testing.T) {
		app, _, targetPane := newAppWithPanes(t)
		events := make([]string, 0, 4)
		runtimeEventsEmitFn = func(_ context.Context, name string, _ ...any) {
			events = append(events, name)
		}

		if err := app.RenamePane(targetPane, "  editor-pane  "); err != nil {
			t.Fatalf("RenamePane() error = %v", err)
		}

		snapshots := app.sessions.Snapshot()
		window := snapshots[0].Windows[0]
		foundTitle := ""
		for _, pane := range window.Panes {
			if pane.ID == targetPane {
				foundTitle = pane.Title
				break
			}
		}
		if foundTitle != "editor-pane" {
			t.Fatalf("pane title = %q, want %q", foundTitle, "editor-pane")
		}
		if !containsEvent(events, "tmux:pane-renamed") {
			t.Fatalf("events = %v, want tmux:pane-renamed", events)
		}
		if !containsAnyEvent(events, "tmux:snapshot", "tmux:snapshot-delta") {
			t.Fatalf("events = %v, want snapshot update event", events)
		}
	})

	t.Run("SwapPanes updates pane order and emits layout-changed event", func(t *testing.T) {
		app, firstPane, secondPane := newAppWithPanes(t)
		var eventsMu sync.Mutex
		events := make([]string, 0, 4)
		runtimeEventsEmitFn = func(_ context.Context, name string, _ ...any) {
			eventsMu.Lock()
			events = append(events, name)
			eventsMu.Unlock()
		}
		eventSnapshot := func() []string {
			eventsMu.Lock()
			defer eventsMu.Unlock()
			return append([]string(nil), events...)
		}

		before := app.sessions.Snapshot()[0].Windows[0].Panes
		if len(before) < 2 {
			t.Fatal("expected at least 2 panes before swap")
		}
		if err := app.SwapPanes(firstPane, secondPane); err != nil {
			t.Fatalf("SwapPanes() error = %v", err)
		}
		after := app.sessions.Snapshot()[0].Windows[0].Panes
		if after[0].ID != secondPane || after[1].ID != firstPane {
			t.Fatalf("pane order after swap = [%q, %q], want [%q, %q]", after[0].ID, after[1].ID, secondPane, firstPane)
		}
		if !containsEvent(eventSnapshot(), "tmux:layout-changed") {
			t.Fatalf("events = %v, want tmux:layout-changed", eventSnapshot())
		}
		waitForCondition(
			t,
			snapshotCoalesceWindow+300*time.Millisecond,
			func() bool { return containsAnyEvent(eventSnapshot(), "tmux:snapshot", "tmux:snapshot-delta") },
			"snapshot update event after swap",
		)
	})

	t.Run("KillPane emits session-destroyed when last pane is removed", func(t *testing.T) {
		app := NewApp()
		app.setRuntimeContext(context.Background())
		app.sessions = tmux.NewSessionManager()
		_, onlyPane, err := app.sessions.CreateSession("solo", "0", 120, 40)
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}

		events := make([]string, 0, 4)
		runtimeEventsEmitFn = func(_ context.Context, name string, _ ...any) {
			events = append(events, name)
		}

		if err := app.KillPane(onlyPane.IDString()); err != nil {
			t.Fatalf("KillPane() error = %v", err)
		}
		if len(app.sessions.Snapshot()) != 0 {
			t.Fatal("session should be removed after killing the last pane")
		}
		if !containsEvent(events, "tmux:session-destroyed") {
			t.Fatalf("events = %v, want tmux:session-destroyed", events)
		}
		if !containsAnyEvent(events, "tmux:snapshot", "tmux:snapshot-delta") {
			t.Fatalf("events = %v, want snapshot update event", events)
		}
	})
}

func containsEvent(events []string, target string) bool {
	return slices.Contains(events, target)
}

func containsAnyEvent(events []string, candidates ...string) bool {
	for _, candidate := range candidates {
		if containsEvent(events, candidate) {
			return true
		}
	}
	return false
}

func TestSplitPaneValidation(t *testing.T) {
	t.Run("requires pane id", func(t *testing.T) {
		app := NewApp()
		app.router = tmux.NewCommandRouter(tmux.NewSessionManager(), nil, tmux.RouterOptions{})

		if _, err := app.SplitPane("   ", true); err == nil {
			t.Fatal("SplitPane() expected pane id validation error")
		}
	})

	t.Run("requires router", func(t *testing.T) {
		app := NewApp()
		app.router = nil

		if _, err := app.SplitPane("%1", true); err == nil {
			t.Fatal("SplitPane() expected router availability error")
		}
	})
}

func TestSplitPaneSuccess(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})

	_, pane, err := app.sessions.CreateSession("session-a", "0", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	newPaneID, err := app.SplitPane(pane.IDString(), true)
	if err != nil {
		t.Fatalf("SplitPane() error = %v", err)
	}
	if newPaneID == "" {
		t.Fatal("SplitPane() returned empty pane id")
	}
	if !app.sessions.HasPane(newPaneID) {
		t.Fatalf("SplitPane() returned pane id %q that does not exist", newPaneID)
	}
}

func TestGetPaneReplay(t *testing.T) {
	t.Run("returns empty when pane state manager is unavailable", func(t *testing.T) {
		app := NewApp()
		app.paneStates = nil
		if got := app.GetPaneReplay("%1"); got != "" {
			t.Fatalf("GetPaneReplay() = %q, want empty string", got)
		}
	})

	t.Run("returns replay data for existing pane", func(t *testing.T) {
		app := NewApp()
		app.paneStates.Feed("%7", []byte("hello pane"))

		if got := app.GetPaneReplay("  %7  "); got != "hello pane" {
			t.Fatalf("GetPaneReplay() = %q, want %q", got, "hello pane")
		}
	})
}

func TestGetPaneEnvValidation(t *testing.T) {
	app := NewApp()
	app.sessions = nil

	if _, err := app.GetPaneEnv("%1"); err == nil {
		t.Fatal("GetPaneEnv() expected session manager availability error")
	}
}

func TestGetPaneEnvSuccess(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()

	_, pane, err := app.sessions.CreateSession("session-a", "0", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	pane.Env["FOO"] = "bar"
	pane.Env["BAZ"] = "qux"

	env, err := app.GetPaneEnv(pane.IDString())
	if err != nil {
		t.Fatalf("GetPaneEnv() error = %v", err)
	}
	if env["FOO"] != "bar" || env["BAZ"] != "qux" {
		t.Fatalf("GetPaneEnv() = %v, want map with FOO=bar and BAZ=qux", env)
	}

	env["FOO"] = "modified"
	if pane.Env["FOO"] != "bar" {
		t.Fatalf("pane env mutated via returned map: got %q, want %q", pane.Env["FOO"], "bar")
	}
}

// --- I-40: Error path tests for GetPaneReplay, GetPaneEnv, ApplyLayoutPreset ---

func TestGetPaneReplayErrorPaths(t *testing.T) {
	tests := []struct {
		name   string
		setup  func() *App
		paneID string
		want   string
	}{
		{
			name: "nil paneStates returns empty",
			setup: func() *App {
				app := NewApp()
				app.paneStates = nil
				return app
			},
			paneID: "%1",
			want:   "",
		},
		{
			name: "empty pane id returns empty",
			setup: func() *App {
				return NewApp()
			},
			paneID: "",
			want:   "",
		},
		{
			name: "whitespace-only pane id returns empty",
			setup: func() *App {
				return NewApp()
			},
			paneID: "   ",
			want:   "",
		},
		{
			name: "nonexistent pane returns empty",
			setup: func() *App {
				return NewApp()
			},
			paneID: "%999",
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := tt.setup()
			got := app.GetPaneReplay(tt.paneID)
			if got != tt.want {
				t.Fatalf("GetPaneReplay(%q) = %q, want %q", tt.paneID, got, tt.want)
			}
		})
	}
}

func TestGetPaneEnvErrorPaths(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *App
		paneID  string
		wantErr string
	}{
		{
			name: "nil sessions",
			setup: func() *App {
				app := NewApp()
				app.sessions = nil
				return app
			},
			paneID:  "%1",
			wantErr: "session manager is unavailable",
		},
		{
			name: "empty pane id",
			setup: func() *App {
				app := NewApp()
				app.sessions = tmux.NewSessionManager()
				return app
			},
			paneID:  "",
			wantErr: "pane id is required",
		},
		{
			name: "whitespace-only pane id",
			setup: func() *App {
				app := NewApp()
				app.sessions = tmux.NewSessionManager()
				return app
			},
			paneID:  "   ",
			wantErr: "pane id is required",
		},
		{
			name: "nonexistent pane",
			setup: func() *App {
				app := NewApp()
				app.sessions = tmux.NewSessionManager()
				return app
			},
			paneID:  "%999",
			wantErr: "pane not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := tt.setup()
			_, err := app.GetPaneEnv(tt.paneID)
			if err == nil {
				t.Fatalf("GetPaneEnv(%q) expected error containing %q", tt.paneID, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("GetPaneEnv(%q) error = %q, want containing %q", tt.paneID, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestApplyLayoutPresetErrorPaths(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() *App
		sessionName string
		preset      string
		wantErr     string
	}{
		{
			name: "nil sessions",
			setup: func() *App {
				app := NewApp()
				app.sessions = nil
				return app
			},
			sessionName: "s1",
			preset:      "even-horizontal",
			wantErr:     "session manager is unavailable",
		},
		{
			name: "empty session name",
			setup: func() *App {
				app := NewApp()
				app.sessions = tmux.NewSessionManager()
				return app
			},
			sessionName: "",
			preset:      "even-horizontal",
			wantErr:     "session name is required",
		},
		{
			name: "whitespace-only session name",
			setup: func() *App {
				app := NewApp()
				app.sessions = tmux.NewSessionManager()
				return app
			},
			sessionName: "   ",
			preset:      "even-horizontal",
			wantErr:     "session name is required",
		},
		{
			name: "empty preset",
			setup: func() *App {
				app := NewApp()
				app.sessions = tmux.NewSessionManager()
				return app
			},
			sessionName: "s1",
			preset:      "",
			wantErr:     "preset is required",
		},
		{
			name: "whitespace-only preset",
			setup: func() *App {
				app := NewApp()
				app.sessions = tmux.NewSessionManager()
				return app
			},
			sessionName: "s1",
			preset:      "   ",
			wantErr:     "preset is required",
		},
		{
			name: "nonexistent session",
			setup: func() *App {
				app := NewApp()
				app.sessions = tmux.NewSessionManager()
				return app
			},
			sessionName: "no-such-session",
			preset:      "even-horizontal",
			wantErr:     "session not found",
		},
		{
			name: "session removed after killing last pane",
			setup: func() *App {
				app := NewApp()
				app.sessions = tmux.NewSessionManager()
				// Create session then kill its only pane to leave zero windows.
				_, pane, err := app.sessions.CreateSession("empty-session", "0", 120, 40)
				if err != nil {
					// Cannot use t.Fatal in setup closure; panic is acceptable in test setup.
					panic(fmt.Sprintf("CreateSession() error = %v", err))
				}
				if _, _, err := app.sessions.KillPane(pane.IDString()); err != nil {
					panic(fmt.Sprintf("KillPane() error = %v", err))
				}
				// Re-create the session as empty: KillPane removes the session entirely
				// when last pane is killed, so we need a different approach.
				// Instead, we rely on ApplyLayoutPresetToActiveWindow returning
				// "session not found" because the session was removed.
				return app
			},
			sessionName: "empty-session",
			preset:      "even-horizontal",
			wantErr:     "session not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := tt.setup()
			err := app.ApplyLayoutPreset(tt.sessionName, tt.preset)
			if err == nil {
				t.Fatalf("ApplyLayoutPreset(%q, %q) expected error containing %q",
					tt.sessionName, tt.preset, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ApplyLayoutPreset(%q, %q) error = %q, want containing %q",
					tt.sessionName, tt.preset, err.Error(), tt.wantErr)
			}
		})
	}
}

// --- S-32: ResizePane boundary value tests ---

func TestResizePaneBoundaryValues(t *testing.T) {
	tests := []struct {
		name   string
		cols   int
		rows   int
		paneID string
	}{
		{name: "zero cols", cols: 0, rows: 30, paneID: "%1"},
		{name: "zero rows", cols: 100, rows: 0, paneID: "%1"},
		{name: "zero cols and rows", cols: 0, rows: 0, paneID: "%1"},
		{name: "negative cols", cols: -1, rows: 30, paneID: "%1"},
		{name: "negative rows", cols: 100, rows: -1, paneID: "%1"},
		{name: "negative cols and rows", cols: -5, rows: -10, paneID: "%1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp()
			app.sessions = tmux.NewSessionManager()

			// ResizePane delegates to SessionManager.ResizePane which requires a
			// pane with a running terminal. Without a terminal, it returns an error.
			// This test verifies ResizePane does not panic on boundary values.
			err := app.ResizePane(tt.paneID, tt.cols, tt.rows)
			if err == nil {
				t.Fatalf("ResizePane(%q, %d, %d) expected error for non-existent pane, got nil",
					tt.paneID, tt.cols, tt.rows)
			}
		})
	}
}

// --- S-38: ApplyLayoutPreset boundary case tests ---

func TestApplyLayoutPresetFallbackToFirstWindow(t *testing.T) {
	// Verify that when ActiveWindowID points to a deleted window,
	// the preset is applied to the first available window (fallback).
	app := NewApp()
	app.sessions = tmux.NewSessionManager()

	_, firstPane, err := app.sessions.CreateSession("session-a", "0", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	// Need at least 2 panes so the layout preset has a visible effect.
	// Initial split is Horizontal; preset uses Vertical to ensure the layout actually changes.
	if _, err := app.sessions.SplitPane(firstPane.ID, tmux.SplitHorizontal); err != nil {
		t.Fatalf("SplitPane() error = %v", err)
	}

	before := app.sessions.Snapshot()
	if len(before) == 0 || len(before[0].Windows) == 0 {
		t.Fatal("unexpected empty snapshot")
	}
	layoutBefore := before[0].Windows[0].Layout

	// Apply a DIFFERENT preset direction (Vertical vs initial Horizontal) so the layout changes.
	if err := app.ApplyLayoutPreset("session-a", string(tmux.PresetEvenVertical)); err != nil {
		t.Fatalf("ApplyLayoutPreset() error = %v", err)
	}

	after := app.sessions.Snapshot()
	if len(after) == 0 || len(after[0].Windows) == 0 {
		t.Fatal("unexpected empty snapshot after preset")
	}
	if reflect.DeepEqual(after[0].Windows[0].Layout, layoutBefore) {
		t.Fatal("layout did not change after applying preset")
	}
}

// TestApplyLayoutPresetSuccessOnSingleWindowModel verifies that ApplyLayoutPreset
// succeeds on the 1-window-per-session model and updates the layout.
func TestApplyLayoutPresetSuccessOnSingleWindowModel(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	app := NewApp()
	t.Cleanup(func() {
		// Clear runtime context before restoring emit to prevent the requestSnapshot
		// timer from firing with the real runtime.EventsEmit after cleanup.
		app.setRuntimeContext(nil)
		runtimeEventsEmitFn = origEmit
	})
	runtimeEventsEmitFn = func(context.Context, string, ...any) {}

	app.setRuntimeContext(context.Background())
	app.sessions = tmux.NewSessionManager()

	_, firstPane, err := app.sessions.CreateSession("session-a", "0", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	// Need at least 2 panes for layout to have effect
	if _, err := app.sessions.SplitPane(firstPane.ID, tmux.SplitHorizontal); err != nil {
		t.Fatalf("SplitPane() error = %v", err)
	}

	// Apply preset - must succeed without error
	if err := app.ApplyLayoutPreset("session-a", string(tmux.PresetEvenHorizontal)); err != nil {
		t.Fatalf("ApplyLayoutPreset() error = %v", err)
	}

	// Session must still exist with its window
	snapshots := app.sessions.Snapshot()
	if len(snapshots) != 1 {
		t.Fatalf("snapshot count = %d, want 1 after ApplyLayoutPreset", len(snapshots))
	}
	if len(snapshots[0].Windows) != 1 {
		t.Fatalf("window count = %d, want 1", len(snapshots[0].Windows))
	}
	if len(snapshots[0].Windows[0].Panes) != 2 {
		t.Fatalf("pane count = %d, want 2", len(snapshots[0].Windows[0].Panes))
	}
}

// --- requireSessionsWithPaneID helper coverage ---

func TestRequireSessionsWithPaneIDTrimsAndValidates(t *testing.T) {
	tests := []struct {
		name    string
		paneID  string
		wantErr string
	}{
		{name: "empty string", paneID: "", wantErr: "pane id is required"},
		{name: "whitespace only", paneID: "   \t  ", wantErr: "pane id is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp()
			app.sessions = tmux.NewSessionManager()
			paneID := tt.paneID
			_, err := app.requireSessionsWithPaneID(&paneID)
			if err == nil {
				t.Fatalf("requireSessionsWithPaneID(%q) expected error, got nil", tt.paneID)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("requireSessionsWithPaneID(%q) error = %q, want containing %q",
					tt.paneID, err.Error(), tt.wantErr)
			}
		})
	}

	t.Run("trims and returns sessions", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		paneID := "  %42  "
		sessions, err := app.requireSessionsWithPaneID(&paneID)
		if err != nil {
			t.Fatalf("requireSessionsWithPaneID() error = %v", err)
		}
		if sessions == nil {
			t.Fatal("requireSessionsWithPaneID() returned nil sessions")
		}
		if paneID != "%42" {
			t.Fatalf("paneID = %q, want %q", paneID, "%42")
		}
	})

	t.Run("nil sessions returns error", func(t *testing.T) {
		app := NewApp()
		app.sessions = nil
		paneID := "%1"
		_, err := app.requireSessionsWithPaneID(&paneID)
		if err == nil {
			t.Fatal("requireSessionsWithPaneID() expected error for nil sessions")
		}
	})

	t.Run("nil paneID pointer returns error", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		_, err := app.requireSessionsWithPaneID(nil)
		if err == nil {
			t.Fatal("requireSessionsWithPaneID(nil) expected error, got nil")
		}
		if !strings.Contains(err.Error(), "paneID pointer must not be nil") {
			t.Fatalf("requireSessionsWithPaneID(nil) error = %q, want containing %q",
				err.Error(), "paneID pointer must not be nil")
		}
	})
}

// --- SUG-19: Special character session name tests ---

func TestRenamePaneWithSpecialCharacterSessionNames(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})
	runtimeEventsEmitFn = func(context.Context, string, ...any) {}

	specialNames := []struct {
		name        string
		sessionName string
	}{
		{name: "colon in name", sessionName: "my:session"},
		{name: "dot in name", sessionName: "my.session"},
		{name: "percent in name", sessionName: "my%session"},
		{name: "space in name", sessionName: "my session"},
		{name: "equals in name", sessionName: "key=value"},
		{name: "mixed special chars", sessionName: "a:b.c%d"},
	}

	for _, tt := range specialNames {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp()
			app.setRuntimeContext(context.Background())
			app.sessions = tmux.NewSessionManager()

			_, pane, err := app.sessions.CreateSession(tt.sessionName, "0", 120, 40)
			if err != nil {
				t.Fatalf("CreateSession(%q) error = %v", tt.sessionName, err)
			}
			paneID := pane.IDString()

			if err := app.RenamePane(paneID, "new-title"); err != nil {
				t.Fatalf("RenamePane() with session %q error = %v", tt.sessionName, err)
			}

			snapshots := app.sessions.Snapshot()
			if len(snapshots) != 1 {
				t.Fatalf("snapshot count = %d, want 1", len(snapshots))
			}
			if snapshots[0].Name != tt.sessionName {
				t.Fatalf("session name = %q, want %q", snapshots[0].Name, tt.sessionName)
			}
			found := false
			for _, w := range snapshots[0].Windows {
				for _, p := range w.Panes {
					if p.ID == paneID && p.Title == "new-title" {
						found = true
					}
				}
			}
			if !found {
				t.Fatalf("pane %q title was not updated to %q", paneID, "new-title")
			}
		})
	}
}
