package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"myT-x/internal/config"
	gitpkg "myT-x/internal/git"
	"myT-x/internal/ipc"
	"myT-x/internal/session"
	"myT-x/internal/testutil"
	"myT-x/internal/tmux"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// NOTE: This file overrides package-level function variables
// (runtimeEventsEmitFn). Do not use t.Parallel() here.
// Use stubExecuteRouterRequest() for send-keys and session router execution stubs.
// Use stubRuntimeEventsEmit() for runtimeEventsEmitFn no-op stubs.

// stubRuntimeEventsEmit is defined in app_test_helpers_test.go.

func runGitInDir(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(append([]string{}, os.Environ()...), "LC_ALL=C", "LC_MESSAGES=C", "LANG=C")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestFindAvailableSessionName(t *testing.T) {
	tests := []struct {
		name             string
		existingSessions []string
		input            string
		want             string
	}{
		{
			name:             "no collision returns as-is",
			existingSessions: nil,
			input:            "test",
			want:             "test",
		},
		{
			name:             "collision appends -2",
			existingSessions: []string{"test"},
			input:            "test",
			want:             "test-2",
		},
		{
			name:             "multiple collisions appends -3",
			existingSessions: []string{"test", "test-2"},
			input:            "test",
			want:             "test-3",
		},
		{
			name:             "different name no collision",
			existingSessions: []string{"other"},
			input:            "test",
			want:             "test",
		},
		{
			name:             "empty string returns as-is",
			existingSessions: nil,
			input:            "",
			want:             "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp()
			app.sessions = tmux.NewSessionManager()
			// Pre-create sessions to simulate collisions.
			for _, name := range tt.existingSessions {
				if _, _, err := app.sessions.CreateSession(name, "0", 120, 40); err != nil {
					t.Fatalf("failed to pre-create session %q: %v", name, err)
				}
			}
			got := app.sessionService.FindAvailableSessionName(tt.input)
			if got != tt.want {
				t.Errorf("findAvailableSessionName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}

	t.Run("returns timestamp suffix when 100 candidates are exhausted", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()

		existing := map[string]struct{}{"test": {}}
		if _, _, err := app.sessions.CreateSession("test", "0", 120, 40); err != nil {
			t.Fatalf("failed to pre-create session %q: %v", "test", err)
		}
		for i := 2; i <= 100; i++ {
			name := fmt.Sprintf("test-%d", i)
			existing[name] = struct{}{}
			if _, _, err := app.sessions.CreateSession(name, "0", 120, 40); err != nil {
				t.Fatalf("failed to pre-create session %q: %v", name, err)
			}
		}

		got := app.sessionService.FindAvailableSessionName("test")
		if _, exists := existing[got]; exists {
			t.Fatalf("findAvailableSessionName() returned existing name %q", got)
		}
		matched, err := regexp.MatchString(`^test-\d+$`, got)
		if err != nil {
			t.Fatalf("regexp compile error: %v", err)
		}
		if !matched {
			t.Fatalf("findAvailableSessionName() = %q, want timestamp suffix format test-<digits>", got)
		}
	})
}

func TestFindAvailableSessionNameBoundary(t *testing.T) {
	t.Run("suffix -2 is the first candidate tried", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		if _, _, err := app.sessions.CreateSession("alpha", "0", 120, 40); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		got := app.sessionService.FindAvailableSessionName("alpha")
		if got != "alpha-2" {
			t.Fatalf("findAvailableSessionName(\"alpha\") = %q, want %q", got, "alpha-2")
		}
	})

	t.Run("suffix at session.MaxSessionNameSuffix boundary", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		// Occupy "boundary" and "boundary-2" through "boundary-99".
		if _, _, err := app.sessions.CreateSession("boundary", "0", 120, 40); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		for i := 2; i <= session.MaxSessionNameSuffix-1; i++ {
			name := fmt.Sprintf("boundary-%d", i)
			if _, _, err := app.sessions.CreateSession(name, "0", 120, 40); err != nil {
				t.Fatalf("CreateSession(%q) error = %v", name, err)
			}
		}
		// "boundary-100" should be the last numeric candidate (session.MaxSessionNameSuffix=100).
		got := app.sessionService.FindAvailableSessionName("boundary")
		if got != fmt.Sprintf("boundary-%d", session.MaxSessionNameSuffix) {
			t.Fatalf("findAvailableSessionName at boundary = %q, want %q",
				got, fmt.Sprintf("boundary-%d", session.MaxSessionNameSuffix))
		}
	})

	t.Run("timestamp fallback is unique from existing names", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		existing := make(map[string]struct{})
		if _, _, err := app.sessions.CreateSession("ts", "0", 120, 40); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		existing["ts"] = struct{}{}
		for i := 2; i <= session.MaxSessionNameSuffix; i++ {
			name := fmt.Sprintf("ts-%d", i)
			existing[name] = struct{}{}
			if _, _, err := app.sessions.CreateSession(name, "0", 120, 40); err != nil {
				t.Fatalf("CreateSession(%q) error = %v", name, err)
			}
		}
		got := app.sessionService.FindAvailableSessionName("ts")
		if _, collision := existing[got]; collision {
			t.Fatalf("timestamp fallback returned existing name %q", got)
		}
		if !strings.HasPrefix(got, "ts-") {
			t.Fatalf("timestamp fallback = %q, want prefix %q", got, "ts-")
		}
	})

	t.Run("session manager unavailable returns original name", func(t *testing.T) {
		app := NewApp()
		app.sessions = nil
		got := app.sessionService.FindAvailableSessionName("fallback")
		if got != "fallback" {
			t.Fatalf("findAvailableSessionName with nil sessions = %q, want %q", got, "fallback")
		}
	})
}

func TestFindSessionByWorktreePath(t *testing.T) {
	tests := []struct {
		name      string
		wtPath    string
		setupPath string
		want      string
	}{
		{
			name:      "finds matching session",
			wtPath:    `C:\Projects\myapp.wt\feature`,
			setupPath: `C:\Projects\myapp.wt\feature`,
			want:      "wt-session",
		},
		{
			name:      "no match returns empty",
			wtPath:    `C:\Projects\other.wt\branch`,
			setupPath: `C:\Projects\myapp.wt\feature`,
			want:      "",
		},
		{
			name:      "path normalization matches",
			wtPath:    `C:\Projects\myapp.wt\feature\`,
			setupPath: `C:\Projects\myapp.wt\feature`,
			want:      "wt-session",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp()
			app.sessions = tmux.NewSessionManager()

			// Create a session and set worktree info.
			if _, _, err := app.sessions.CreateSession("wt-session", "0", 120, 40); err != nil {
				t.Fatalf("failed to create session: %v", err)
			}
			if err := app.sessions.SetWorktreeInfo("wt-session", &tmux.SessionWorktreeInfo{
				Path:       tt.setupPath,
				RepoPath:   `C:\Projects\myapp`,
				BranchName: "feature",
				BaseBranch: "main",
				IsDetached: false,
			}); err != nil {
				t.Fatalf("failed to set worktree info: %v", err)
			}

			got := app.sessionService.FindSessionByWorktreePath(tt.wtPath)
			if got != tt.want {
				t.Errorf("findSessionByWorktreePath(%q) = %q, want %q", tt.wtPath, got, tt.want)
			}
		})
	}
}

func TestCheckWorktreePathConflict(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()

	// No sessions -> no conflict.
	got := app.CheckWorktreePathConflict(`C:\Projects\myapp.wt\feature`)
	if got != "" {
		t.Errorf("expected empty conflict, got %q", got)
	}

	// Create a session with worktree.
	if _, _, err := app.sessions.CreateSession("sess1", "0", 120, 40); err != nil {
		t.Fatal(err)
	}
	if err := app.sessions.SetWorktreeInfo("sess1", &tmux.SessionWorktreeInfo{
		Path:       `C:\Projects\myapp.wt\feature`,
		RepoPath:   `C:\Projects\myapp`,
		BranchName: "feature",
		BaseBranch: "main",
		IsDetached: false,
	}); err != nil {
		t.Fatal(err)
	}

	// Now the path should conflict.
	got = app.CheckWorktreePathConflict(`C:\Projects\myapp.wt\feature`)
	if got != "sess1" {
		t.Errorf("expected conflict with sess1, got %q", got)
	}

	// Whitespace trimming.
	got = app.CheckWorktreePathConflict(`  C:\Projects\myapp.wt\feature  `)
	if got != "sess1" {
		t.Errorf("expected whitespace-trimmed conflict with sess1, got %q", got)
	}
}

func TestGetCurrentBranch(t *testing.T) {
	testutil.SkipIfNoGit(t)
	app := NewApp()

	dir := testutil.CreateTempGitRepo(t)
	branch, err := app.GetCurrentBranch(dir)
	if err != nil {
		t.Fatalf("GetCurrentBranch() error = %v", err)
	}
	// Git init creates either "main" or "master" depending on configuration.
	if branch == "" {
		t.Error("expected non-empty branch name")
	}
}

func TestPromoteWorktreeToBranchSuccess(t *testing.T) {
	repoPath := testutil.CreateTempGitRepo(t)
	runGitInDir(t, repoPath, "checkout", "--detach")

	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	if _, _, err := app.sessions.CreateSession("session-a", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if err := app.sessions.SetWorktreeInfo("session-a", &tmux.SessionWorktreeInfo{
		Path:       repoPath,
		RepoPath:   repoPath,
		BranchName: "",
		BaseBranch: "HEAD",
		IsDetached: true,
	}); err != nil {
		t.Fatalf("SetWorktreeInfo() error = %v", err)
	}

	if err := app.PromoteWorktreeToBranch("session-a", "feature/promoted"); err != nil {
		t.Fatalf("PromoteWorktreeToBranch() error = %v", err)
	}

	info, err := app.sessions.GetWorktreeInfo("session-a")
	if err != nil {
		t.Fatalf("GetWorktreeInfo() error = %v", err)
	}
	if info == nil {
		t.Fatal("GetWorktreeInfo() returned nil after successful promotion")
	}
	if info.BranchName != "feature/promoted" {
		t.Fatalf("worktree branch = %q, want %q", info.BranchName, "feature/promoted")
	}
	if info.IsDetached {
		t.Fatal("worktree should not be detached after promotion")
	}

	if current := runGitInDir(t, repoPath, "branch", "--show-current"); current != "feature/promoted" {
		t.Fatalf("current git branch = %q, want %q", current, "feature/promoted")
	}
}

func TestCommitAndPushWorktreeSuccess(t *testing.T) {
	repoPath := testutil.CreateTempGitRepo(t)
	remoteRoot := t.TempDir()
	remotePath := filepath.Join(remoteRoot, "origin.git")
	runGitInDir(t, remoteRoot, "init", "--bare", remotePath)

	branchName := runGitInDir(t, repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	runGitInDir(t, repoPath, "remote", "add", "origin", remotePath)
	runGitInDir(t, repoPath, "push", "-u", "origin", branchName)

	if err := os.WriteFile(filepath.Join(repoPath, "feature.txt"), []byte("feature-change"), 0o644); err != nil {
		t.Fatalf("write feature.txt: %v", err)
	}

	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	if _, _, err := app.sessions.CreateSession("session-a", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if err := app.sessions.SetWorktreeInfo("session-a", &tmux.SessionWorktreeInfo{
		Path:       repoPath,
		RepoPath:   repoPath,
		BranchName: branchName,
		BaseBranch: "",
		IsDetached: false,
	}); err != nil {
		t.Fatalf("SetWorktreeInfo() error = %v", err)
	}

	if err := app.CommitAndPushWorktree("session-a", "add feature file", true); err != nil {
		t.Fatalf("CommitAndPushWorktree() error = %v", err)
	}

	if status := runGitInDir(t, repoPath, "status", "--porcelain"); status != "" {
		t.Fatalf("working tree should be clean after commit, got status %q", status)
	}

	localHead := runGitInDir(t, repoPath, "rev-parse", "HEAD")
	remoteHead := runGitInDir(t, repoPath, "--git-dir", remotePath, "rev-parse", "refs/heads/"+branchName)
	if localHead != remoteHead {
		t.Fatalf("remote head = %q, want %q", remoteHead, localHead)
	}
}

func TestCommitAndPushWorktreePushOnlyWhenCommitMessageEmpty(t *testing.T) {
	repoPath := testutil.CreateTempGitRepo(t)
	remoteRoot := t.TempDir()
	remotePath := filepath.Join(remoteRoot, "origin.git")
	runGitInDir(t, remoteRoot, "init", "--bare", remotePath)

	branchName := runGitInDir(t, repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	runGitInDir(t, repoPath, "remote", "add", "origin", remotePath)
	runGitInDir(t, repoPath, "push", "-u", "origin", branchName)

	// Create one local commit that is not yet pushed.
	if err := os.WriteFile(filepath.Join(repoPath, "push-only.txt"), []byte("push-only"), 0o644); err != nil {
		t.Fatalf("write push-only.txt: %v", err)
	}
	runGitInDir(t, repoPath, "add", "push-only.txt")
	runGitInDir(t, repoPath, "commit", "-m", "local commit for push-only test")

	localHeadBefore := runGitInDir(t, repoPath, "rev-parse", "HEAD")
	remoteHeadBefore := runGitInDir(t, repoPath, "--git-dir", remotePath, "rev-parse", "refs/heads/"+branchName)
	if localHeadBefore == remoteHeadBefore {
		t.Fatal("expected local branch to be ahead before push-only test")
	}

	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	if _, _, err := app.sessions.CreateSession("session-a", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if err := app.sessions.SetWorktreeInfo("session-a", &tmux.SessionWorktreeInfo{
		Path:       repoPath,
		RepoPath:   repoPath,
		BranchName: branchName,
		BaseBranch: "",
		IsDetached: false,
	}); err != nil {
		t.Fatalf("SetWorktreeInfo() error = %v", err)
	}

	if err := app.CommitAndPushWorktree("session-a", "   ", true); err != nil {
		t.Fatalf("CommitAndPushWorktree() push-only error = %v", err)
	}

	remoteHeadAfter := runGitInDir(t, repoPath, "--git-dir", remotePath, "rev-parse", "refs/heads/"+branchName)
	if remoteHeadAfter != localHeadBefore {
		t.Fatalf("remote head after push-only = %q, want %q", remoteHeadAfter, localHeadBefore)
	}
}

func TestCreateSessionWithWorktreeValidation(t *testing.T) {
	t.Run("returns error when session manager is unavailable", func(t *testing.T) {
		app := NewApp()
		app.sessions = nil
		app.router = tmux.NewCommandRouter(nil, nil, tmux.RouterOptions{})
		app.configState.SetSnapshot(config.DefaultConfig())

		if _, err := app.CreateSessionWithWorktree(t.TempDir(), "session-a", WorktreeSessionOptions{
			BranchName: "feature/test",
		}); err == nil {
			t.Fatal("CreateSessionWithWorktree() expected session manager availability error")
		}
	})

	t.Run("returns error when router is unavailable", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		app.router = nil
		app.configState.SetSnapshot(config.DefaultConfig())

		if _, err := app.CreateSessionWithWorktree(t.TempDir(), "session-a", WorktreeSessionOptions{
			BranchName: "feature/test",
		}); err == nil {
			t.Fatal("CreateSessionWithWorktree() expected router availability error")
		}
	})

	t.Run("returns error when worktree feature is disabled", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		cfg := config.DefaultConfig()
		cfg.Worktree.Enabled = false
		app.configState.SetSnapshot(cfg)

		if _, err := app.CreateSessionWithWorktree(t.TempDir(), "session-a", WorktreeSessionOptions{
			BranchName: "feature/test",
		}); err == nil {
			t.Fatal("CreateSessionWithWorktree() expected disabled feature error")
		}
	})

	t.Run("sanitizes empty session name to fallback", func(t *testing.T) {
		repoPath := testutil.CreateTempGitRepo(t)
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
		cfg := config.DefaultConfig()
		cfg.Worktree.Enabled = true
		app.configState.SetSnapshot(cfg)
		stubNewSessionCommandSuccess(t, app)

		snapshot, err := app.CreateSessionWithWorktree(repoPath, "   ", WorktreeSessionOptions{
			BranchName: "feature/test",
		})
		if err != nil {
			t.Fatalf("CreateSessionWithWorktree() error = %v", err)
		}
		if snapshot.Name != "worktree-session" {
			t.Fatalf("CreateSessionWithWorktree() session name = %q, want %q (fallback)", snapshot.Name, "worktree-session")
		}
	})

	t.Run("returns error when branch name is empty", func(t *testing.T) {
		repoPath := testutil.CreateTempGitRepo(t)
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
		app.configState.SetSnapshot(config.DefaultConfig())

		if _, err := app.CreateSessionWithWorktree(repoPath, "session-a", WorktreeSessionOptions{}); err == nil {
			t.Fatal("CreateSessionWithWorktree() expected branch validation error")
		}
	})

	t.Run("continues when pull-before-create fails", func(t *testing.T) {
		repoPath := testutil.CreateTempGitRepo(t)
		app := NewApp()
		app.setRuntimeContext(context.Background())
		app.sessions = tmux.NewSessionManager()
		app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
		app.configState.SetSnapshot(config.DefaultConfig())

		events := make([]string, 0, 4)
		origEmit := runtimeEventsEmitFn
		runtimeEventsEmitFn = func(_ context.Context, name string, _ ...any) {
			events = append(events, name)
		}
		t.Cleanup(func() { runtimeEventsEmitFn = origEmit })

		routerCalls := 0
		stubExecuteRouterRequest(t, app, func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
			routerCalls++
			if req.Command != "new-session" {
				return ipc.TmuxResponse{ExitCode: 1, Stderr: "unexpected command"}
			}
			sessionName, _ := req.Flags["-s"].(string)
			if sessionName == "" {
				return ipc.TmuxResponse{ExitCode: 1, Stderr: "missing session name"}
			}
			if _, _, err := app.sessions.CreateSession(sessionName, "0", 120, 40); err != nil {
				return ipc.TmuxResponse{ExitCode: 1, Stderr: err.Error()}
			}
			return ipc.TmuxResponse{ExitCode: 0, Stdout: sessionName}
		})

		snapshot, err := app.CreateSessionWithWorktree(repoPath, "session-a", WorktreeSessionOptions{
			BranchName:       "feature/pull-before-create",
			PullBeforeCreate: true,
		})
		if err != nil {
			t.Fatalf("CreateSessionWithWorktree() error = %v", err)
		}
		if snapshot.Name != "session-a" {
			t.Fatalf("snapshot.Name = %q, want %q", snapshot.Name, "session-a")
		}
		if routerCalls != 1 {
			t.Fatalf("router call count = %d, want 1 when pull falls back to local state", routerCalls)
		}
		if got := len(app.sessions.Snapshot()); got != 1 {
			t.Fatalf("session count = %d, want 1 after pull fallback", got)
		}
		foundPullFailedEvent := slices.Contains(events, "worktree:pull-failed")
		if !foundPullFailedEvent {
			t.Fatalf("CreateSessionWithWorktree() events = %v, want worktree:pull-failed event", events)
		}
	})
}

func TestCreateSessionWithExistingWorktreeValidation(t *testing.T) {
	t.Run("returns error when session manager is unavailable", func(t *testing.T) {
		app := NewApp()
		app.sessions = nil
		app.router = tmux.NewCommandRouter(nil, nil, tmux.RouterOptions{})
		app.configState.SetSnapshot(config.DefaultConfig())

		if _, err := app.CreateSessionWithExistingWorktree(t.TempDir(), "session-a", t.TempDir(), CreateSessionOptions{}); err == nil {
			t.Fatal("CreateSessionWithExistingWorktree() expected session manager availability error")
		}
	})

	t.Run("returns error when router is unavailable", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		app.router = nil
		app.configState.SetSnapshot(config.DefaultConfig())

		if _, err := app.CreateSessionWithExistingWorktree(t.TempDir(), "session-a", t.TempDir(), CreateSessionOptions{}); err == nil {
			t.Fatal("CreateSessionWithExistingWorktree() expected router availability error")
		}
	})

	t.Run("returns error when worktree feature is disabled", func(t *testing.T) {
		repoPath := testutil.CreateTempGitRepo(t)
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
		cfg := config.DefaultConfig()
		cfg.Worktree.Enabled = false
		app.configState.SetSnapshot(cfg)

		if _, err := app.CreateSessionWithExistingWorktree(repoPath, "session-a", repoPath, CreateSessionOptions{}); err == nil {
			t.Fatal("CreateSessionWithExistingWorktree() expected disabled feature error")
		}
	})

	t.Run("returns error when repository path is empty", func(t *testing.T) {
		repoPath := testutil.CreateTempGitRepo(t)
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
		app.configState.SetSnapshot(config.DefaultConfig())

		_, err := app.CreateSessionWithExistingWorktree("   ", "session-a", repoPath, CreateSessionOptions{})
		if err == nil {
			t.Fatal("CreateSessionWithExistingWorktree() expected repository-path validation error")
		}
		if !strings.Contains(err.Error(), "repository path is required") {
			t.Fatalf("error = %v, want repository path validation message", err)
		}
	})

	t.Run("returns error when worktree path is empty", func(t *testing.T) {
		repoPath := testutil.CreateTempGitRepo(t)
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
		app.configState.SetSnapshot(config.DefaultConfig())

		_, err := app.CreateSessionWithExistingWorktree(repoPath, "session-a", "   ", CreateSessionOptions{})
		if err == nil {
			t.Fatal("CreateSessionWithExistingWorktree() expected worktree-path validation error")
		}
		if !strings.Contains(err.Error(), "worktree path is required") {
			t.Fatalf("error = %v, want worktree path validation message", err)
		}
	})

	t.Run("sanitizes empty session name to fallback", func(t *testing.T) {
		repoPath := testutil.CreateTempGitRepo(t)
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
		cfg := config.DefaultConfig()
		cfg.Worktree.Enabled = true
		app.configState.SetSnapshot(cfg)
		stubNewSessionCommandSuccess(t, app)

		snapshot, err := app.CreateSessionWithExistingWorktree(repoPath, "   ", repoPath, CreateSessionOptions{})
		if err != nil {
			t.Fatalf("CreateSessionWithExistingWorktree() error = %v", err)
		}
		if snapshot.Name != "worktree-session" {
			t.Fatalf("CreateSessionWithExistingWorktree() session name = %q, want %q (fallback)", snapshot.Name, "worktree-session")
		}
	})

	t.Run("returns error when worktree path is already in use", func(t *testing.T) {
		repoPath := testutil.CreateTempGitRepo(t)
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
		app.configState.SetSnapshot(config.DefaultConfig())
		if _, _, err := app.sessions.CreateSession("using-worktree", "0", 120, 40); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		if err := app.sessions.SetWorktreeInfo("using-worktree", &tmux.SessionWorktreeInfo{
			Path:       repoPath,
			RepoPath:   repoPath,
			BranchName: "main",
		}); err != nil {
			t.Fatalf("SetWorktreeInfo() error = %v", err)
		}

		_, err := app.CreateSessionWithExistingWorktree(repoPath, "session-a", repoPath, CreateSessionOptions{})
		if err == nil {
			t.Fatal("CreateSessionWithExistingWorktree() expected conflict error")
		}
		if !strings.Contains(err.Error(), "already in use by session") {
			t.Fatalf("error = %v, want conflict message", err)
		}
	})
}

func TestCreateSessionForDirectoryReturnsErrorWhenTmuxReturnsEmptyName(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})

	stubExecuteRouterRequest(t, app, func(_ *tmux.CommandRouter, _ ipc.TmuxRequest) ipc.TmuxResponse {
		return ipc.TmuxResponse{ExitCode: 0, Stdout: "   "}
	})

	if _, err := app.sessionService.CreateSessionForDirectory(t.TempDir(), "session-a", session.CreateSessionOptions{}); err == nil {
		t.Fatal("CreateSessionForDirectory() expected empty-name error")
	}
}

func TestCreateSessionForDirectoryRollsBackWhenTmuxReturnsEmptyName(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})

	var killSessionCalled bool
	stubExecuteRouterRequest(t, app, func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
		switch req.Command {
		case "new-session":
			return ipc.TmuxResponse{ExitCode: 0, Stdout: "   "}
		case "kill-session":
			target, _ := req.Flags["-t"].(string)
			if strings.TrimSpace(target) != "session-a" {
				t.Fatalf("rollback kill target = %q, want %q", target, "session-a")
			}
			killSessionCalled = true
			return ipc.TmuxResponse{ExitCode: 0}
		default:
			return ipc.TmuxResponse{ExitCode: 1, Stderr: "unexpected command"}
		}
	})

	if _, err := app.sessionService.CreateSessionForDirectory(t.TempDir(), "session-a", session.CreateSessionOptions{}); err == nil {
		t.Fatal("CreateSessionForDirectory() expected empty-name error")
	}
	if !killSessionCalled {
		t.Fatal("expected rollback kill-session call when tmux returns empty created name")
	}
}

func TestCreateSessionWithWorktreeSuccess(t *testing.T) {
	repoPath := testutil.CreateTempGitRepo(t)
	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
	app.configState.SetSnapshot(config.DefaultConfig())

	events := make([]string, 0, 4)
	origEmit := runtimeEventsEmitFn
	runtimeEventsEmitFn = func(_ context.Context, name string, _ ...any) {
		events = append(events, name)
	}
	t.Cleanup(func() { runtimeEventsEmitFn = origEmit })

	stubExecuteRouterRequest(t, app, func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
		switch req.Command {
		case "new-session":
			sessionName, _ := req.Flags["-s"].(string)
			sessionName = strings.TrimSpace(sessionName)
			if sessionName == "" {
				return ipc.TmuxResponse{ExitCode: 1, Stderr: "missing session name\n"}
			}
			if _, _, err := app.sessions.CreateSession(sessionName, "0", 120, 40); err != nil {
				return ipc.TmuxResponse{ExitCode: 1, Stderr: err.Error() + "\n"}
			}
			return ipc.TmuxResponse{ExitCode: 0, Stdout: sessionName + "\n"}
		default:
			return ipc.TmuxResponse{ExitCode: 1, Stderr: "unexpected command\n"}
		}
	})

	snapshot, err := app.CreateSessionWithWorktree(
		repoPath,
		"session-a",
		WorktreeSessionOptions{BranchName: "feature/session-a"},
	)
	if err != nil {
		t.Fatalf("CreateSessionWithWorktree() error = %v", err)
	}
	if snapshot.Name != "session-a" {
		t.Fatalf("snapshot.Name = %q, want %q", snapshot.Name, "session-a")
	}
	foundSnapshotEvent := false
	for _, name := range events {
		if name == "tmux:snapshot" || name == "tmux:snapshot-delta" {
			foundSnapshotEvent = true
			break
		}
	}
	if !foundSnapshotEvent {
		t.Fatalf("CreateSessionWithWorktree() events = %v, want snapshot event", events)
	}

	info, err := app.sessions.GetWorktreeInfo(snapshot.Name)
	if err != nil {
		t.Fatalf("GetWorktreeInfo() error = %v", err)
	}
	if info == nil {
		t.Fatal("GetWorktreeInfo() returned nil")
	}
	if info.RepoPath != repoPath {
		t.Fatalf("info.RepoPath = %q, want %q", info.RepoPath, repoPath)
	}
	if info.BranchName != "feature/session-a" {
		t.Fatalf("info.BranchName = %q, want %q", info.BranchName, "feature/session-a")
	}
	if info.IsDetached {
		t.Fatal("info.IsDetached = true, want false")
	}
	if strings.TrimSpace(info.Path) == "" {
		t.Fatal("info.Path is empty")
	}
	if _, statErr := os.Stat(info.Path); statErr != nil {
		t.Fatalf("worktree path stat error: %v", statErr)
	}

	if currentBranch := runGitInDir(t, info.Path, "branch", "--show-current"); currentBranch != "feature/session-a" {
		t.Fatalf("worktree current branch = %q, want %q", currentBranch, "feature/session-a")
	}
}

func TestCreateSessionWithWorktreeRollsBackWorktreeWhenSessionCreationFails(t *testing.T) {
	repoPath := testutil.CreateTempGitRepo(t)
	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
	app.configState.SetSnapshot(config.DefaultConfig())

	capturedWorktreePath := ""
	stubExecuteRouterRequest(t, app, func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
		if req.Command != "new-session" {
			return ipc.TmuxResponse{ExitCode: 1, Stderr: "unexpected command\n"}
		}
		if worktreePath, ok := req.Flags["-c"].(string); ok {
			capturedWorktreePath = strings.TrimSpace(worktreePath)
		}
		return ipc.TmuxResponse{ExitCode: 1, Stderr: "simulated session creation failure\n"}
	})

	_, err := app.CreateSessionWithWorktree(repoPath, "session-a", WorktreeSessionOptions{
		BranchName: "feature/rollback-worktree",
	})
	if err == nil {
		t.Fatal("CreateSessionWithWorktree() expected session creation error")
	}
	if !strings.Contains(err.Error(), "failed to create session") {
		t.Fatalf("CreateSessionWithWorktree() error = %v, want session creation failure", err)
	}
	if strings.TrimSpace(capturedWorktreePath) == "" {
		t.Fatal("expected captured worktree path from new-session request")
	}
	if _, statErr := os.Stat(capturedWorktreePath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("rollback should remove worktree path %q, stat error = %v", capturedWorktreePath, statErr)
	}
	if got := len(app.sessions.Snapshot()); got != 0 {
		t.Fatalf("session count after rollback = %d, want 0", got)
	}

	repo, openErr := gitpkg.Open(repoPath)
	if openErr != nil {
		t.Fatalf("Open() error = %v", openErr)
	}
	branches, listErr := repo.ListBranches()
	if listErr != nil {
		t.Fatalf("ListBranches() error = %v", listErr)
	}
	for _, branch := range branches {
		if branch == "feature/rollback-worktree" {
			t.Fatalf("rollback branch %q should be cleaned up", branch)
		}
	}
}

func TestCreateSessionWithWorktreeEmitsCleanupFailureEventWhenRollbackFails(t *testing.T) {
	repoPath := testutil.CreateTempGitRepo(t)
	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
	app.configState.SetSnapshot(config.DefaultConfig())

	capturedWorktreePath := ""
	stubExecuteRouterRequest(t, app, func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
		if req.Command != "new-session" {
			return ipc.TmuxResponse{ExitCode: 1, Stderr: "unexpected command\n"}
		}
		if worktreePath, ok := req.Flags["-c"].(string); ok {
			capturedWorktreePath = strings.TrimSpace(worktreePath)
		}
		_ = os.RemoveAll(filepath.Join(repoPath, ".git"))
		return ipc.TmuxResponse{ExitCode: 1, Stderr: "simulated session creation failure\n"}
	})

	var cleanupPayload map[string]any
	origEmit := runtimeEventsEmitFn
	runtimeEventsEmitFn = func(_ context.Context, name string, data ...any) {
		if name != "worktree:cleanup-failed" || len(data) == 0 {
			return
		}
		payload, ok := data[0].(map[string]any)
		if ok {
			cleanupPayload = payload
		}
	}
	t.Cleanup(func() { runtimeEventsEmitFn = origEmit })

	_, err := app.CreateSessionWithWorktree(repoPath, "session-a", WorktreeSessionOptions{
		BranchName: "feature/rollback-failed",
	})
	if err == nil {
		t.Fatal("CreateSessionWithWorktree() expected rollback failure error")
	}
	if !strings.Contains(err.Error(), "worktree rollback also failed") {
		t.Fatalf("CreateSessionWithWorktree() error = %v, want rollback failure details", err)
	}
	if cleanupPayload == nil {
		t.Fatal("expected worktree:cleanup-failed event payload")
	}
	if got := cleanupPayload["sessionName"]; got != "session-a" {
		t.Fatalf("cleanup payload sessionName = %v, want session-a", got)
	}
	if strings.TrimSpace(capturedWorktreePath) == "" {
		t.Fatal("expected captured worktree path from new-session request")
	}
	if got := cleanupPayload["path"]; got != capturedWorktreePath {
		t.Fatalf("cleanup payload path = %v, want %q", got, capturedWorktreePath)
	}
}

func TestCreateSessionWithWorktreeEnableAgentTeamSetsEnvVars(t *testing.T) {
	repoPath := testutil.CreateTempGitRepo(t)
	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
	app.configState.SetSnapshot(config.DefaultConfig())

	var capturedReq ipc.TmuxRequest
	stubExecuteRouterRequest(t, app, func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
		capturedReq = req
		sessionName, _ := req.Flags["-s"].(string)
		sessionName = strings.TrimSpace(sessionName)
		if sessionName == "" {
			return ipc.TmuxResponse{ExitCode: 1, Stderr: "missing session name\n"}
		}
		if _, _, err := app.sessions.CreateSession(sessionName, "0", 120, 40); err != nil {
			return ipc.TmuxResponse{ExitCode: 1, Stderr: err.Error() + "\n"}
		}
		return ipc.TmuxResponse{ExitCode: 0, Stdout: sessionName + "\n"}
	})

	snapshot, err := app.CreateSessionWithWorktree(
		repoPath,
		"team-worktree",
		WorktreeSessionOptions{
			BranchName:      "feature/team-worktree",
			EnableAgentTeam: true,
		},
	)
	if err != nil {
		t.Fatalf("CreateSessionWithWorktree() error = %v", err)
	}

	wantEnv := session.AgentTeamEnvVars(snapshot.Name)
	if len(capturedReq.Env) != len(wantEnv) {
		t.Fatalf("captured env count = %d, want %d", len(capturedReq.Env), len(wantEnv))
	}
	for key, wantValue := range wantEnv {
		if got := capturedReq.Env[key]; got != wantValue {
			t.Fatalf("captured env[%q] = %q, want %q", key, got, wantValue)
		}
	}
}

func TestCleanupWorktreeValidation(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	if _, _, err := app.sessions.CreateSession("session-a", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	if err := app.CleanupWorktree("session-a"); err == nil {
		t.Fatal("CleanupWorktree() expected error for session without worktree")
	}
}

func TestCleanupWorktreeSuccessClearsMetadata(t *testing.T) {
	repoPath := testutil.CreateTempGitRepo(t)
	wtParent := t.TempDir()
	wtParent = testutil.ResolvePath(wtParent)
	worktreePath := filepath.Join(wtParent, "cleanup-worktree")
	runGitInDir(t, repoPath, "worktree", "add", "-b", "feature/cleanup", worktreePath)

	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	if _, _, err := app.sessions.CreateSession("session-a", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if err := app.sessions.SetWorktreeInfo("session-a", &tmux.SessionWorktreeInfo{
		Path:       worktreePath,
		RepoPath:   repoPath,
		BranchName: "feature/cleanup",
	}); err != nil {
		t.Fatalf("SetWorktreeInfo() error = %v", err)
	}

	if err := app.CleanupWorktree("session-a"); err != nil {
		t.Fatalf("CleanupWorktree() error = %v", err)
	}
	if _, statErr := os.Stat(worktreePath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("worktree path should be removed, stat err = %v", statErr)
	}
	info, err := app.sessions.GetWorktreeInfo("session-a")
	if err != nil {
		t.Fatalf("GetWorktreeInfo() error = %v", err)
	}
	if info != nil {
		t.Fatalf("worktree info = %#v, want nil after cleanup", info)
	}
}

func TestCleanupWorktreeKeepsPushedBranch(t *testing.T) {
	repoPath := testutil.CreateTempGitRepo(t)
	defaultBranch := runGitInDir(t, repoPath, "rev-parse", "--abbrev-ref", "HEAD")

	remoteRoot := t.TempDir()
	remotePath := filepath.Join(remoteRoot, "origin.git")
	runGitInDir(t, remoteRoot, "init", "--bare", remotePath)

	runGitInDir(t, repoPath, "remote", "add", "origin", remotePath)
	runGitInDir(t, repoPath, "push", "-u", "origin", defaultBranch)

	wtParent := testutil.ResolvePath(t.TempDir())
	worktreePath := filepath.Join(wtParent, "cleanup-worktree-pushed")
	runGitInDir(t, repoPath, "worktree", "add", "-b", "feature/cleanup-pushed", worktreePath)
	runGitInDir(t, worktreePath, "push", "-u", "origin", "feature/cleanup-pushed")

	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	if _, _, err := app.sessions.CreateSession("session-a", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if err := app.sessions.SetWorktreeInfo("session-a", &tmux.SessionWorktreeInfo{
		Path:       worktreePath,
		RepoPath:   repoPath,
		BranchName: "feature/cleanup-pushed",
	}); err != nil {
		t.Fatalf("SetWorktreeInfo() error = %v", err)
	}

	if err := app.CleanupWorktree("session-a"); err != nil {
		t.Fatalf("CleanupWorktree() error = %v", err)
	}
	if _, statErr := os.Stat(worktreePath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("worktree path should be removed, stat err = %v", statErr)
	}

	// The branch was pushed and should remain available.
	runGitInDir(t, repoPath, "rev-parse", "--verify", "refs/heads/feature/cleanup-pushed")

	branches, err := app.ListBranches(repoPath)
	if err != nil {
		t.Fatalf("ListBranches() error = %v", err)
	}
	found := slices.Contains(branches, "feature/cleanup-pushed")
	if !found {
		t.Fatalf("pushed branch should remain selectable, got %v", branches)
	}
}

func TestListWorktreesByRepoReturnsMainAndLinkedWorktree(t *testing.T) {
	repoPath := testutil.CreateTempGitRepo(t)
	wtParent := testutil.ResolvePath(t.TempDir())
	worktreePath := filepath.Join(wtParent, "listed-worktree")
	runGitInDir(t, repoPath, "worktree", "add", "-b", "feature/listed", worktreePath)

	app := NewApp()
	worktrees, err := app.ListWorktreesByRepo(repoPath)
	if err != nil {
		t.Fatalf("ListWorktreesByRepo() error = %v", err)
	}
	if len(worktrees) < 2 {
		t.Fatalf("worktree count = %d, want at least 2", len(worktrees))
	}

	hasMain := false
	hasLinked := false
	for _, wt := range worktrees {
		if filepath.Clean(wt.Path) == filepath.Clean(repoPath) && wt.IsMain {
			hasMain = true
		}
		if filepath.Clean(wt.Path) == filepath.Clean(worktreePath) {
			hasLinked = true
		}
	}
	if !hasMain {
		t.Fatalf("main worktree not found in %+v", worktrees)
	}
	if !hasLinked {
		t.Fatalf("linked worktree %q not found in %+v", worktreePath, worktrees)
	}
}

func TestCreateSessionWithExistingWorktreeLogsRollbackKillSessionFailure(t *testing.T) {
	repoPath := testutil.CreateTempGitRepo(t)

	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
	app.configState.SetSnapshot(config.DefaultConfig())

	var requests []ipc.TmuxRequest
	stubExecuteRouterRequest(t, app, func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
		requests = append(requests, req)
		switch req.Command {
		case "new-session":
			return ipc.TmuxResponse{ExitCode: 0, Stdout: "existing-wt\n"}
		case "kill-session":
			return ipc.TmuxResponse{ExitCode: 1, Stderr: "simulated kill failure\n"}
		default:
			return ipc.TmuxResponse{ExitCode: 1, Stderr: "unexpected command\n"}
		}
	})

	logBuf := testutil.CaptureLogBuffer(t, slog.LevelDebug)

	_, err := app.CreateSessionWithExistingWorktree(repoPath, "existing-wt", repoPath, CreateSessionOptions{})
	if err == nil {
		t.Fatal("CreateSessionWithExistingWorktree() expected SetWorktreeInfo error")
	}
	if !strings.Contains(err.Error(), "failed to set worktree info") {
		t.Fatalf("CreateSessionWithExistingWorktree() error = %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("execute call count = %d, want 2", len(requests))
	}
	if requests[0].Command != "new-session" {
		t.Fatalf("first request command = %q, want new-session", requests[0].Command)
	}
	if requests[1].Command != "kill-session" {
		t.Fatalf("second request command = %q, want kill-session", requests[1].Command)
	}
	if !strings.Contains(logBuf.String(), "rollback kill-session failed") {
		t.Fatalf("log output = %q, want rollback failure warning", logBuf.String())
	}
}

func TestCreateSessionWithExistingWorktreeRollsBackSessionOnSetWorktreeFailure(t *testing.T) {
	repoPath := testutil.CreateTempGitRepo(t)

	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
	app.configState.SetSnapshot(config.DefaultConfig())

	var requests []ipc.TmuxRequest
	stubExecuteRouterRequest(t, app, func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
		requests = append(requests, req)
		switch req.Command {
		case "new-session":
			return ipc.TmuxResponse{ExitCode: 0, Stdout: "existing-wt\n"}
		case "kill-session":
			return ipc.TmuxResponse{ExitCode: 0}
		default:
			return ipc.TmuxResponse{ExitCode: 1, Stderr: "unexpected command\n"}
		}
	})

	_, err := app.CreateSessionWithExistingWorktree(repoPath, "existing-wt", repoPath, CreateSessionOptions{})
	if err == nil {
		t.Fatal("CreateSessionWithExistingWorktree() expected SetWorktreeInfo error")
	}
	if !strings.Contains(err.Error(), "failed to set worktree info") {
		t.Fatalf("CreateSessionWithExistingWorktree() error = %v", err)
	}
	if strings.Contains(err.Error(), "rollback also failed") {
		t.Fatalf("CreateSessionWithExistingWorktree() error = %v, want successful rollback", err)
	}
	if len(requests) != 2 {
		t.Fatalf("execute call count = %d, want 2", len(requests))
	}
	if requests[0].Command != "new-session" {
		t.Fatalf("first request command = %q, want new-session", requests[0].Command)
	}
	if requests[1].Command != "kill-session" {
		t.Fatalf("second request command = %q, want kill-session", requests[1].Command)
	}
}

func TestCreateSessionWithExistingWorktreeReturnsStatErrorForInvalidPath(t *testing.T) {
	repoPath := testutil.CreateTempGitRepo(t)

	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
	app.configState.SetSnapshot(config.DefaultConfig())

	_, err := app.CreateSessionWithExistingWorktree(repoPath, "existing-wt", "\x00", CreateSessionOptions{})
	if err == nil {
		t.Fatal("CreateSessionWithExistingWorktree() expected stat error for invalid worktree path")
	}
	if !strings.Contains(err.Error(), "failed to stat worktree path") {
		t.Fatalf("error = %v, want stat error message", err)
	}
}

func TestWorktreePublicAPIsValidation(t *testing.T) {
	t.Run("CheckWorktreeStatus returns HasWorktree false for non-worktree session", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		if _, _, err := app.sessions.CreateSession("session-a", "0", 120, 40); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}

		status, err := app.CheckWorktreeStatus("session-a")
		if err != nil {
			t.Fatalf("CheckWorktreeStatus() error = %v", err)
		}
		if status.HasWorktree {
			t.Fatal("CheckWorktreeStatus() expected HasWorktree=false")
		}
	})

	t.Run("PromoteWorktreeToBranch returns error when session has no worktree", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		if _, _, err := app.sessions.CreateSession("session-a", "0", 120, 40); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}

		if err := app.PromoteWorktreeToBranch("session-a", "feature/new"); err == nil {
			t.Fatal("PromoteWorktreeToBranch() expected no-worktree error")
		}
	})

	t.Run("CommitAndPushWorktree returns error when session has no worktree", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		if _, _, err := app.sessions.CreateSession("session-a", "0", 120, 40); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}

		if err := app.CommitAndPushWorktree("session-a", "message", true); err == nil {
			t.Fatal("CommitAndPushWorktree() expected no-worktree error")
		}
	})

	t.Run("ListWorktreesByRepo returns error for non-git directory", func(t *testing.T) {
		app := NewApp()
		if _, err := app.ListWorktreesByRepo(t.TempDir()); err == nil {
			t.Fatal("ListWorktreesByRepo() expected error for non-git directory")
		}
	})

	t.Run("ListBranches returns error for non-git directory", func(t *testing.T) {
		app := NewApp()
		if _, err := app.ListBranches(t.TempDir()); err == nil {
			t.Fatal("ListBranches() expected error for non-git directory")
		}
	})

	t.Run("IsGitRepository distinguishes git and non-git directories", func(t *testing.T) {
		repoPath := testutil.CreateTempGitRepo(t)
		app := NewApp()
		if !app.IsGitRepository(repoPath) {
			t.Fatal("IsGitRepository() expected true for git repository")
		}
		if app.IsGitRepository(t.TempDir()) {
			t.Fatal("IsGitRepository() expected false for non-git directory")
		}
	})
}

func TestWorktreePublicAPIsRejectEmptySessionName(t *testing.T) {
	app := NewApp()

	if err := app.PromoteWorktreeToBranch("   ", "feature/new"); err == nil {
		t.Fatal("PromoteWorktreeToBranch() expected session-name validation error")
	}
	if err := app.CommitAndPushWorktree("   ", "message", true); err == nil {
		t.Fatal("CommitAndPushWorktree() expected session-name validation error")
	}
	if _, err := app.CheckWorktreeStatus("   "); err == nil {
		t.Fatal("CheckWorktreeStatus() expected session-name validation error")
	}
	if err := app.CleanupWorktree("   "); err == nil {
		t.Fatal("CleanupWorktree() expected session-name validation error")
	}
}

func TestPromoteWorktreeToBranchRejectsInvalidBranchName(t *testing.T) {
	repoPath := testutil.CreateTempGitRepo(t)
	runGitInDir(t, repoPath, "checkout", "--detach")

	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	if _, _, err := app.sessions.CreateSession("session-a", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if err := app.sessions.SetWorktreeInfo("session-a", &tmux.SessionWorktreeInfo{
		Path:       repoPath,
		RepoPath:   repoPath,
		BranchName: "",
		BaseBranch: "HEAD",
		IsDetached: true,
	}); err != nil {
		t.Fatalf("SetWorktreeInfo() error = %v", err)
	}

	if err := app.PromoteWorktreeToBranch("session-a", "invalid branch name"); err == nil {
		t.Fatal("PromoteWorktreeToBranch() expected invalid-branch error")
	}
}

func TestCreateSessionWithExistingWorktreeEnableAgentTeamSetsEnvVars(t *testing.T) {
	repoPath := testutil.CreateTempGitRepo(t)

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
	app.configState.SetSnapshot(config.DefaultConfig())

	events := make([]string, 0, 4)
	origEmit := runtimeEventsEmitFn
	runtimeEventsEmitFn = func(_ context.Context, name string, _ ...any) {
		events = append(events, name)
	}
	t.Cleanup(func() { runtimeEventsEmitFn = origEmit })

	var capturedReq ipc.TmuxRequest
	stubExecuteRouterRequest(t, app, func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
		capturedReq = req
		switch req.Command {
		case "new-session":
			sessionName, _ := req.Flags["-s"].(string)
			sessionName = strings.TrimSpace(sessionName)
			if sessionName == "" {
				return ipc.TmuxResponse{ExitCode: 1, Stderr: "missing session name\n"}
			}
			if _, _, err := app.sessions.CreateSession(sessionName, "0", 120, 40); err != nil {
				return ipc.TmuxResponse{ExitCode: 1, Stderr: err.Error() + "\n"}
			}
			return ipc.TmuxResponse{ExitCode: 0, Stdout: sessionName + "\n"}
		default:
			return ipc.TmuxResponse{ExitCode: 1, Stderr: "unexpected command\n"}
		}
	})

	snapshot, err := app.CreateSessionWithExistingWorktree(repoPath, "existing-wt-team", repoPath, CreateSessionOptions{EnableAgentTeam: true})
	if err != nil {
		t.Fatalf("CreateSessionWithExistingWorktree() error = %v", err)
	}

	wantEnv := session.AgentTeamEnvVars(snapshot.Name)
	if len(capturedReq.Env) != len(wantEnv) {
		t.Fatalf("captured env count = %d, want %d", len(capturedReq.Env), len(wantEnv))
	}
	for key, wantValue := range wantEnv {
		if got := capturedReq.Env[key]; got != wantValue {
			t.Fatalf("captured env[%q] = %q, want %q", key, got, wantValue)
		}
	}

	foundSnapshotEvent := false
	for _, name := range events {
		if name == "tmux:snapshot" || name == "tmux:snapshot-delta" {
			foundSnapshotEvent = true
			break
		}
	}
	if !foundSnapshotEvent {
		t.Fatalf("CreateSessionWithExistingWorktree() events = %v, want snapshot event", events)
	}
}

func TestListBranchesHidesWorktreeOnlyLocalBranches(t *testing.T) {
	repoPath := testutil.CreateTempGitRepo(t)
	defaultBranch := runGitInDir(t, repoPath, "rev-parse", "--abbrev-ref", "HEAD")

	remoteRoot := t.TempDir()
	remotePath := filepath.Join(remoteRoot, "origin.git")
	runGitInDir(t, remoteRoot, "init", "--bare", remotePath)

	runGitInDir(t, repoPath, "remote", "add", "origin", remotePath)
	runGitInDir(t, repoPath, "push", "-u", "origin", defaultBranch)

	localOnlyWorktree := filepath.Join(testutil.ResolvePath(t.TempDir()), "worktree-local-only")
	runGitInDir(t, repoPath, "worktree", "add", "-b", "feature/local-only", localOnlyWorktree)
	runGitInDir(t, repoPath, "worktree", "remove", localOnlyWorktree)

	pushedWorktree := filepath.Join(testutil.ResolvePath(t.TempDir()), "worktree-pushed")
	runGitInDir(t, repoPath, "worktree", "add", "-b", "feature/pushed", pushedWorktree)
	runGitInDir(t, pushedWorktree, "push", "-u", "origin", "feature/pushed")
	runGitInDir(t, repoPath, "worktree", "remove", pushedWorktree)

	app := NewApp()
	branches, err := app.ListBranches(repoPath)
	if err != nil {
		t.Fatalf("ListBranches() error = %v", err)
	}

	hasBranch := func(target string) bool {
		return slices.Contains(branches, target)
	}

	if !hasBranch(defaultBranch) {
		t.Fatalf("base branch %q should be listed, got %v", defaultBranch, branches)
	}
	if !hasBranch("feature/pushed") {
		t.Fatalf("pushed branch should be listed, got %v", branches)
	}
	if hasBranch("feature/local-only") {
		t.Fatalf("worktree-only local branch should be hidden, got %v", branches)
	}
}

func TestCheckWorktreeStatusNormalScenarios(t *testing.T) {
	t.Run("clean and dirty worktree states", func(t *testing.T) {
		repoPath := testutil.CreateTempGitRepo(t)
		branchName := runGitInDir(t, repoPath, "rev-parse", "--abbrev-ref", "HEAD")

		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		if _, _, err := app.sessions.CreateSession("session-a", "0", 120, 40); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		if err := app.sessions.SetWorktreeInfo("session-a", &tmux.SessionWorktreeInfo{
			Path:       repoPath,
			RepoPath:   repoPath,
			BranchName: branchName,
			BaseBranch: "",
			IsDetached: false,
		}); err != nil {
			t.Fatalf("SetWorktreeInfo() error = %v", err)
		}

		cleanStatus, err := app.CheckWorktreeStatus("session-a")
		if err != nil {
			t.Fatalf("CheckWorktreeStatus(clean) error = %v", err)
		}
		if !cleanStatus.HasWorktree {
			t.Fatal("CheckWorktreeStatus(clean) expected HasWorktree=true")
		}
		if cleanStatus.HasUncommitted {
			t.Fatal("CheckWorktreeStatus(clean) expected HasUncommitted=false")
		}
		if cleanStatus.IsDetached {
			t.Fatal("CheckWorktreeStatus(clean) expected IsDetached=false")
		}

		if err := os.WriteFile(filepath.Join(repoPath, "dirty.txt"), []byte("dirty"), 0o644); err != nil {
			t.Fatalf("write dirty file: %v", err)
		}
		dirtyStatus, err := app.CheckWorktreeStatus("session-a")
		if err != nil {
			t.Fatalf("CheckWorktreeStatus(dirty) error = %v", err)
		}
		if !dirtyStatus.HasUncommitted {
			t.Fatal("CheckWorktreeStatus(dirty) expected HasUncommitted=true")
		}
	})

	t.Run("detached worktree reports detached metadata", func(t *testing.T) {
		repoPath := testutil.CreateTempGitRepo(t)
		runGitInDir(t, repoPath, "checkout", "--detach")

		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		if _, _, err := app.sessions.CreateSession("session-detached", "0", 120, 40); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		if err := app.sessions.SetWorktreeInfo("session-detached", &tmux.SessionWorktreeInfo{
			Path:       repoPath,
			RepoPath:   repoPath,
			BranchName: "",
			BaseBranch: "HEAD",
			IsDetached: true,
		}); err != nil {
			t.Fatalf("SetWorktreeInfo() error = %v", err)
		}

		status, err := app.CheckWorktreeStatus("session-detached")
		if err != nil {
			t.Fatalf("CheckWorktreeStatus(detached) error = %v", err)
		}
		if !status.HasWorktree {
			t.Fatal("CheckWorktreeStatus(detached) expected HasWorktree=true")
		}
		if !status.IsDetached {
			t.Fatal("CheckWorktreeStatus(detached) expected IsDetached=true")
		}
		if status.BranchName != "" {
			t.Fatalf("CheckWorktreeStatus(detached) branch = %q, want empty", status.BranchName)
		}
	})
}

func TestWorktreeAPIsRequireSessionManager(t *testing.T) {
	app := NewApp()
	app.sessions = nil

	if err := app.PromoteWorktreeToBranch("session-a", "feature/new"); err == nil {
		t.Fatal("PromoteWorktreeToBranch() expected session manager availability error")
	}
	if err := app.CleanupWorktree("session-a"); err == nil {
		t.Fatal("CleanupWorktree() expected session manager availability error")
	}
	if _, err := app.CheckWorktreeStatus("session-a"); err == nil {
		t.Fatal("CheckWorktreeStatus() expected session manager availability error")
	}
	if err := app.CommitAndPushWorktree("session-a", "message", true); err == nil {
		t.Fatal("CommitAndPushWorktree() expected session manager availability error")
	}
	if got := app.sessionService.FindSessionByWorktreePath(`C:\Projects\myapp.wt\feature`); got != "" {
		t.Fatalf("findSessionByWorktreePath() = %q, want empty when sessions is nil", got)
	}
}
