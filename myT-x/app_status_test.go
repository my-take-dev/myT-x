package main

import (
	"regexp"
	"strings"
	"testing"

	"myT-x/internal/tmux"
)

func TestBuildStatusLine(t *testing.T) {
	t.Run("returns fallback when sessions is nil", func(t *testing.T) {
		app := NewApp()
		app.sessions = nil

		line := app.BuildStatusLine()
		if !strings.Contains(line, "[セッションなし]") {
			t.Fatalf("BuildStatusLine() = %q, want fallback message", line)
		}
	})

	t.Run("returns formatted line for active session", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		if _, _, err := app.sessions.CreateSession("alpha", "0", 120, 40); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		app.setActiveSessionName("alpha")

		line := app.BuildStatusLine()
		if !strings.Contains(line, "[alpha]") {
			t.Fatalf("BuildStatusLine() = %q, want active session name", line)
		}
	})

	t.Run("does not mutate active session as side effect", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		if _, _, err := app.sessions.CreateSession("alpha", "0", 120, 40); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		app.setActiveSessionName("missing")

		_ = app.BuildStatusLine()

		if got := app.getActiveSessionName(); got != "missing" {
			t.Fatalf("active session = %q, want %q", got, "missing")
		}
	})

	t.Run("renders active session when worktree metadata exists", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		if _, _, err := app.sessions.CreateSession("alpha", "0", 120, 40); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		if err := app.sessions.SetWorktreeInfo("alpha", &tmux.SessionWorktreeInfo{
			Path:       `C:\Projects\repo.wt\alpha`,
			RepoPath:   `C:\Projects\repo`,
			BranchName: "alpha",
			BaseBranch: "main",
			IsDetached: false,
		}); err != nil {
			t.Fatalf("SetWorktreeInfo() error = %v", err)
		}
		app.setActiveSessionName("alpha")

		line := app.BuildStatusLine()
		if !strings.Contains(line, "[alpha]") {
			t.Fatalf("BuildStatusLine() = %q, want active session name", line)
		}
	})

	t.Run("includes active pane title and window id", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		_, pane, err := app.sessions.CreateSession("alpha", "0", 120, 40)
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		if _, err := app.sessions.SplitPane(pane.ID, tmux.SplitHorizontal); err != nil {
			t.Fatalf("SplitPane() error = %v", err)
		}
		if _, err := app.sessions.RenamePane(pane.IDString(), "editor"); err != nil {
			t.Fatalf("RenamePane() error = %v", err)
		}
		app.setActiveSessionName("alpha")

		line := app.BuildStatusLine()
		if !strings.Contains(line, "<0>") {
			t.Fatalf("BuildStatusLine() = %q, want window id marker <0>", line)
		}
		if !strings.Contains(line, "\"editor\"") {
			t.Fatalf("BuildStatusLine() = %q, want active pane title", line)
		}
	})

	t.Run("uses HH:MM time format suffix", func(t *testing.T) {
		app := NewApp()
		app.sessions = nil

		line := app.BuildStatusLine()
		matched, err := regexp.MatchString(`\|\s\d{2}:\d{2}$`, line)
		if err != nil {
			t.Fatalf("regex compile error: %v", err)
		}
		if !matched {
			t.Fatalf("BuildStatusLine() = %q, want HH:MM suffix", line)
		}
	})
}

func TestResolvePaneTitleUsesActivePaneIndex(t *testing.T) {
	panes := []tmux.PaneSnapshot{
		{ID: "%7", Index: 1, Title: "by-id-only", Active: true},
		{ID: "%42", Index: 7, Title: "by-index", Active: false},
	}

	got := resolvePaneTitle(panes, 7)
	if got != "by-index" {
		t.Fatalf("resolvePaneTitle() = %q, want %q", got, "by-index")
	}
}

func TestResolvePaneTitleEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		panes    []tmux.PaneSnapshot
		activePN int
		want     string
	}{
		{
			// activePN does not match any pane.Index; fall back to pane with Active=true.
			name: "activePN mismatch falls back to Active=true pane",
			panes: []tmux.PaneSnapshot{
				{ID: "%1", Index: 0, Title: "inactive-pane", Active: false},
				{ID: "%2", Index: 1, Title: "active-pane", Active: true},
			},
			activePN: 99,
			want:     "active-pane",
		},
		{
			// activePN mismatch, no Active=true pane: return the first pane with a non-empty Title.
			name: "activePN mismatch and no Active pane returns first titled pane",
			panes: []tmux.PaneSnapshot{
				{ID: "%1", Index: 0, Title: "", Active: false},
				{ID: "%2", Index: 1, Title: "only-titled", Active: false},
			},
			activePN: 99,
			want:     "only-titled",
		},
		{
			// activePN mismatch, no Active=true, no titled pane: return first pane ID as last resort.
			name: "activePN mismatch and no titled pane falls back to first pane ID",
			panes: []tmux.PaneSnapshot{
				{ID: "%3", Index: 0, Title: "", Active: false},
				{ID: "%4", Index: 1, Title: "", Active: false},
			},
			activePN: 99,
			want:     "%3",
		},
		{
			// activePN matches but matched pane has no title; fall back to any titled pane.
			name: "matched pane has empty title falls back to other titled pane",
			panes: []tmux.PaneSnapshot{
				{ID: "%1", Index: 0, Title: "", Active: true},
				{ID: "%2", Index: 1, Title: "other-title", Active: false},
			},
			activePN: 0,
			want:     "other-title",
		},
		{
			name:     "empty panes returns empty string",
			panes:    nil,
			activePN: 0,
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolvePaneTitle(tt.panes, tt.activePN)
			if got != tt.want {
				t.Fatalf("resolvePaneTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveActiveWindowSnapshotUsesActiveWindowID(t *testing.T) {
	windows := []tmux.WindowSnapshot{
		{ID: 10, Name: "first"},
		{ID: 20, Name: "second"},
	}

	got := resolveActiveWindowSnapshot(windows, 20)
	if got == nil {
		t.Fatal("resolveActiveWindowSnapshot() = nil, want non-nil")
	}
	if got.ID != 20 {
		t.Fatalf("resolveActiveWindowSnapshot().ID = %d, want %d", got.ID, 20)
	}
}

func TestResolveActiveWindowSnapshotFallsBackToFirst(t *testing.T) {
	windows := []tmux.WindowSnapshot{
		{ID: 10, Name: "first"},
		{ID: 20, Name: "second"},
	}

	got := resolveActiveWindowSnapshot(windows, 999)
	if got == nil {
		t.Fatal("resolveActiveWindowSnapshot() = nil, want non-nil")
	}
	if got.ID != 10 {
		t.Fatalf("resolveActiveWindowSnapshot().ID = %d, want %d", got.ID, 10)
	}
}

func TestResolveActiveWindowSnapshotReturnsNilOnEmptyWindows(t *testing.T) {
	got := resolveActiveWindowSnapshot(nil, 1)
	if got != nil {
		t.Fatalf("resolveActiveWindowSnapshot() = %#v, want nil", got)
	}
}
