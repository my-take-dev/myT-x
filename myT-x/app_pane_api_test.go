package main

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"myT-x/internal/tmux"
)

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
		runtimeEventsEmitFn = func(_ context.Context, name string, _ ...interface{}) {
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
		runtimeEventsEmitFn = func(_ context.Context, name string, _ ...interface{}) {
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
		runtimeEventsEmitFn = func(_ context.Context, name string, _ ...interface{}) {
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
		runtimeEventsEmitFn = func(_ context.Context, name string, _ ...interface{}) {
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
	for _, event := range events {
		if event == target {
			return true
		}
	}
	return false
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
