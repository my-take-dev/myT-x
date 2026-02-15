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
