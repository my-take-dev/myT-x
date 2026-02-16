package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"myT-x/internal/config"
	gitpkg "myT-x/internal/git"
	"myT-x/internal/ipc"
	"myT-x/internal/terminal"
	"myT-x/internal/testutil"
	"myT-x/internal/tmux"
)

func TestCreateRenameKillSessionValidation(t *testing.T) {
	t.Run("CreateSession returns error when root path is empty", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
		if _, err := app.CreateSession("   ", "session-a", false); err == nil {
			t.Fatal("CreateSession() expected root path validation error")
		}
	})

	t.Run("CreateSession returns error when session name is empty", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
		if _, err := app.CreateSession(t.TempDir(), "   ", false); err == nil {
			t.Fatal("CreateSession() expected session name validation error")
		}
	})

	t.Run("CreateSession returns error when session manager is unavailable", func(t *testing.T) {
		app := NewApp()
		if _, err := app.CreateSession(t.TempDir(), "session-a", false); err == nil {
			t.Fatal("CreateSession() expected session manager availability error")
		}
	})

	t.Run("CreateSession returns error when router is unavailable", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		if _, err := app.CreateSession(t.TempDir(), "session-a", false); err == nil {
			t.Fatal("CreateSession() expected router availability error")
		}
	})

	t.Run("RenameSession returns error when session manager is unavailable", func(t *testing.T) {
		app := NewApp()
		if err := app.RenameSession("old", "new"); err == nil {
			t.Fatal("RenameSession() expected session manager availability error")
		}
	})

	t.Run("RenameSession returns error when old name is empty", func(t *testing.T) {
		app := NewApp()
		if err := app.RenameSession("   ", "new"); err == nil {
			t.Fatal("RenameSession() expected old-name validation error")
		}
	})

	t.Run("RenameSession returns error when new name is empty", func(t *testing.T) {
		app := NewApp()
		if err := app.RenameSession("old", "   "); err == nil {
			t.Fatal("RenameSession() expected new-name validation error")
		}
	})

	t.Run("KillSession returns error when session name is empty", func(t *testing.T) {
		app := NewApp()
		if err := app.KillSession("   ", false); err == nil {
			t.Fatal("KillSession() expected session name validation error")
		}
	})

	t.Run("KillSession returns error when session manager is unavailable", func(t *testing.T) {
		app := NewApp()
		if err := app.KillSession("session-a", false); err == nil {
			t.Fatal("KillSession() expected session manager availability error")
		}
	})

	t.Run("KillSession returns error when router is unavailable", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		if _, _, err := app.sessions.CreateSession("session-a", "0", 120, 40); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		if err := app.KillSession("session-a", false); err == nil {
			t.Fatal("KillSession() expected router availability error")
		}
	})
}

func TestCreateSessionReturnsErrorWhenTmuxReturnsEmptyCreatedName(t *testing.T) {
	origExecute := executeRouterRequestFn
	t.Cleanup(func() {
		executeRouterRequestFn = origExecute
	})

	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})

	executeRouterRequestFn = func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
		if req.Command != "new-session" {
			return ipc.TmuxResponse{ExitCode: 1, Stderr: "unexpected command"}
		}
		return ipc.TmuxResponse{ExitCode: 0, Stdout: "   "}
	}

	if _, err := app.CreateSession(t.TempDir(), "session-a", false); err == nil {
		t.Fatal("CreateSession() expected error when tmux returns empty created name")
	}
}

func TestCreateSessionRollsBackWhenStoreRootPathFails(t *testing.T) {
	origExecute := executeRouterRequestFn
	t.Cleanup(func() {
		executeRouterRequestFn = origExecute
	})

	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})

	var rollbackTargets []string
	executeRouterRequestFn = func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
		switch req.Command {
		case "new-session":
			return ipc.TmuxResponse{ExitCode: 0, Stdout: "missing-session\n"}
		case "kill-session":
			target, _ := req.Flags["-t"].(string)
			rollbackTargets = append(rollbackTargets, strings.TrimSpace(target))
			return ipc.TmuxResponse{ExitCode: 0}
		default:
			return ipc.TmuxResponse{ExitCode: 1, Stderr: "unexpected command"}
		}
	}

	_, err := app.CreateSession(t.TempDir(), "session-a", false)
	if err == nil {
		t.Fatal("CreateSession() expected storeRootPath error")
	}
	if !strings.Contains(err.Error(), "failed to set root path for conflict detection") {
		t.Fatalf("CreateSession() error = %v, want storeRootPath failure", err)
	}
	if len(rollbackTargets) != 1 {
		t.Fatalf("rollback kill-session count = %d, want 1", len(rollbackTargets))
	}
	if rollbackTargets[0] != "missing-session" {
		t.Fatalf("rollback target = %q, want %q", rollbackTargets[0], "missing-session")
	}
}

func TestCreateSessionLogsGitMetadataFailures(t *testing.T) {
	repoPath := testutil.CreateTempGitRepo(t)
	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})

	logBuf := testutil.CaptureLogBuffer(t, slog.LevelDebug)

	originalExecute := executeRouterRequestFn
	t.Cleanup(func() {
		executeRouterRequestFn = originalExecute
	})

	executeRouterRequestFn = func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
		if req.Command != "new-session" {
			return ipc.TmuxResponse{ExitCode: 1, Stderr: "unexpected command"}
		}
		sessionName, _ := req.Flags["-s"].(string)
		if _, _, err := app.sessions.CreateSession(sessionName, "0", 120, 40); err != nil {
			return ipc.TmuxResponse{ExitCode: 1, Stderr: err.Error()}
		}
		return ipc.TmuxResponse{ExitCode: 0, Stdout: sessionName}
	}

	// Keep git-dir detection passing but force CurrentBranch() to fail.
	// This validates that metadata-enrichment failures are logged without
	// breaking session creation.
	headPath := filepath.Join(repoPath, ".git", "HEAD")
	if err := os.WriteFile(headPath, []byte("ref: refs/heads/does-not-exist\n"), 0o644); err != nil {
		t.Fatalf("failed to corrupt HEAD for metadata-failure test: %v", err)
	}

	if _, err := app.CreateSession(repoPath, "session-a", false); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	snapshots := app.sessions.Snapshot()
	if len(snapshots) != 1 {
		t.Fatalf("snapshot count = %d, want 1", len(snapshots))
	}
	if snapshots[0].Name != "session-a" && !strings.HasPrefix(snapshots[0].Name, "session-a") {
		t.Fatalf("created session name = %q, want session-a prefix", snapshots[0].Name)
	}
	if !strings.Contains(logBuf.String(), "failed to read git branch for session metadata") {
		t.Fatalf("expected git metadata failure log, got logs: %q", logBuf.String())
	}
}

func TestCreateSessionWithAgentTeamEnvVars(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})

	originalExecute := executeRouterRequestFn
	t.Cleanup(func() {
		executeRouterRequestFn = originalExecute
	})

	var capturedReq ipc.TmuxRequest
	executeRouterRequestFn = func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
		capturedReq = req
		sessionName, _ := req.Flags["-s"].(string)
		if _, _, err := app.sessions.CreateSession(sessionName, "0", 120, 40); err != nil {
			return ipc.TmuxResponse{ExitCode: 1, Stderr: err.Error()}
		}
		return ipc.TmuxResponse{ExitCode: 0, Stdout: sessionName}
	}

	if _, err := app.CreateSession(t.TempDir(), "team-session", true); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	want := agentTeamEnvVars("team-session")
	if len(capturedReq.Env) != len(want) {
		t.Fatalf("CreateSession() env count = %d, want %d", len(capturedReq.Env), len(want))
	}
	for key, wantValue := range want {
		if got := capturedReq.Env[key]; got != wantValue {
			t.Fatalf("CreateSession() env[%q] = %q, want %q", key, got, wantValue)
		}
	}
}

func TestRenameSessionUpdatesSnapshotAndEmitsSnapshotEvent(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.sessions = tmux.NewSessionManager()
	if _, _, err := app.sessions.CreateSession("old-name", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	app.setActiveSessionName("old-name")

	var events []string
	runtimeEventsEmitFn = func(_ context.Context, name string, _ ...interface{}) {
		events = append(events, name)
	}

	if err := app.RenameSession("old-name", "new-name"); err != nil {
		t.Fatalf("RenameSession() error = %v", err)
	}

	if got := app.getActiveSessionName(); got != "new-name" {
		t.Fatalf("active session = %q, want %q", got, "new-name")
	}

	var foundOld, foundNew bool
	for _, snapshot := range app.sessions.Snapshot() {
		if snapshot.Name == "old-name" {
			foundOld = true
		}
		if snapshot.Name == "new-name" {
			foundNew = true
		}
	}
	if foundOld {
		t.Fatal("old-name should not remain in snapshot after rename")
	}
	if !foundNew {
		t.Fatal("new-name should exist in snapshot after rename")
	}

	foundSnapshotEvent := false
	for _, name := range events {
		if name == "tmux:snapshot" || name == "tmux:snapshot-delta" {
			foundSnapshotEvent = true
			break
		}
	}
	if !foundSnapshotEvent {
		t.Fatalf("RenameSession() events = %v, want snapshot event", events)
	}
}

func TestGetSessionEnvReturnsSessionEnvironment(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()

	session, _, err := app.sessions.CreateSession("session-a", "0", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	session.Env["FOO"] = "bar"
	session.Env["BAZ"] = "qux"

	env, err := app.GetSessionEnv("session-a")
	if err != nil {
		t.Fatalf("GetSessionEnv() error = %v", err)
	}
	if env["FOO"] != "bar" || env["BAZ"] != "qux" {
		t.Fatalf("GetSessionEnv() = %v, want map with FOO=bar and BAZ=qux", env)
	}

	env["FOO"] = "modified"
	if session.Env["FOO"] != "bar" {
		t.Fatalf("session env mutated via returned map: got %q, want %q", session.Env["FOO"], "bar")
	}
}

func TestIsWorktreeCleanForRemoval(t *testing.T) {
	dir := testutil.CreateTempGitRepo(t)
	app := NewApp()

	t.Run("clean worktree returns true", func(t *testing.T) {
		if !app.isWorktreeCleanForRemoval(dir) {
			t.Error("expected clean repo to return true")
		}
	})

	t.Run("dirty worktree returns false", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("dirty"), 0o644); err != nil {
			t.Fatal(err)
		}
		if app.isWorktreeCleanForRemoval(dir) {
			t.Error("expected dirty repo to return false")
		}
	})

	t.Run("nonexistent path returns false", func(t *testing.T) {
		if app.isWorktreeCleanForRemoval("/nonexistent/path/12345") {
			t.Error("expected nonexistent path to return false")
		}
	})
}

func TestIsGitRepositoryAPI(t *testing.T) {
	testutil.SkipIfNoGit(t)
	app := NewApp()

	t.Run("valid git repo", func(t *testing.T) {
		dir := testutil.CreateTempGitRepo(t)
		if !app.IsGitRepository(dir) {
			t.Error("expected IsGitRepository to return true")
		}
	})

	t.Run("non-git directory", func(t *testing.T) {
		dir := t.TempDir()
		if app.IsGitRepository(dir) {
			t.Error("expected IsGitRepository to return false")
		}
	})

	t.Run("trims whitespace", func(t *testing.T) {
		dir := testutil.CreateTempGitRepo(t)
		if !app.IsGitRepository("  " + dir + "  ") {
			t.Error("expected IsGitRepository to handle whitespace")
		}
	})
}

func TestListBranchesAPI(t *testing.T) {
	testutil.SkipIfNoGit(t)
	app := NewApp()

	dir := testutil.CreateTempGitRepo(t)
	branches, err := app.ListBranches(dir)
	if err != nil {
		t.Fatalf("ListBranches() error = %v", err)
	}
	if len(branches) == 0 {
		t.Error("expected at least one branch")
	}
}

// TestCopyConfigPathTraversalDeep tests deeper path traversal scenarios.
func TestCopyConfigPathTraversalDeep(t *testing.T) {
	tests := []struct {
		name     string
		file     string
		wantSkip bool
	}{
		{"simple traversal", "../secret", true},
		{"nested traversal", "sub/../../secret", true},
		{"absolute path unix", "/etc/passwd", true},
		{"normal relative", "config/.env", false},
		{"dot file", ".env", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoDir := t.TempDir()
			wtDir := t.TempDir()

			// Create the source file so we can detect if it gets copied.
			if !tt.wantSkip {
				srcDir := filepath.Dir(filepath.Join(repoDir, tt.file))
				if err := os.MkdirAll(srcDir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(repoDir, tt.file), []byte("data"), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			failures := copyConfigFilesToWorktree(repoDir, wtDir, []string{tt.file})

			if tt.wantSkip {
				// Should be skipped, not in failures (traversal entries are skipped silently).
				// Verify nothing was written to wtDir outside expected paths.
				dstPath := filepath.Join(wtDir, filepath.Clean(tt.file))
				if _, err := os.Stat(dstPath); err == nil {
					t.Errorf("path traversal file %q should not have been copied", tt.file)
				}
			} else {
				if len(failures) != 0 {
					t.Errorf("expected no failures for valid file %q, got %v", tt.file, failures)
				}
			}
		})
	}
}

func TestFindSessionByRootPath(t *testing.T) {
	tests := []struct {
		name      string
		dir       string
		setupPath string
		want      string
	}{
		{
			name:      "exact match",
			dir:       `C:\Projects\myapp`,
			setupPath: `C:\Projects\myapp`,
			want:      "test-session",
		},
		{
			name:      "trailing separator normalization",
			dir:       `C:\Projects\myapp\`,
			setupPath: `C:\Projects\myapp`,
			want:      "test-session",
		},
		{
			name:      "no match returns empty",
			dir:       `C:\Projects\other`,
			setupPath: `C:\Projects\myapp`,
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp()
			app.sessions = tmux.NewSessionManager()

			if _, _, err := app.sessions.CreateSession("test-session", "0", 120, 40); err != nil {
				t.Fatalf("failed to create session: %v", err)
			}
			if err := app.sessions.SetRootPath("test-session", tt.setupPath); err != nil {
				t.Fatalf("failed to set root path: %v", err)
			}

			got := app.findSessionByRootPath(tt.dir)
			if got != tt.want {
				t.Errorf("findSessionByRootPath(%q) = %q, want %q", tt.dir, got, tt.want)
			}
		})
	}
}

func TestFindSessionByRootPathSkipsWorktreeSessions(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()

	// Worktreeセッションを作成（RootPath=リポジトリ、WorktreePath=実作業ディレクトリ）
	if _, _, err := app.sessions.CreateSession("wt-session", "0", 120, 40); err != nil {
		t.Fatal(err)
	}
	if err := app.sessions.SetRootPath("wt-session", `C:\Projects\myapp`); err != nil {
		t.Fatal(err)
	}
	if err := app.sessions.SetWorktreeInfo("wt-session", &tmux.SessionWorktreeInfo{
		Path:       `C:\Projects\myapp.wt\feature`,
		RepoPath:   `C:\Projects\myapp`,
		BranchName: "feature",
		BaseBranch: "main",
		IsDetached: false,
	}); err != nil {
		t.Fatal(err)
	}

	// Worktreeセッションはスキップされるため、コンフリクトなし。
	if got := app.findSessionByRootPath(`C:\Projects\myapp`); got != "" {
		t.Errorf("worktree session should be skipped, got %q", got)
	}
}

func TestCheckDirectoryConflictSkipsWorktreeSessions(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()

	// Worktreeセッションのみ → コンフリクトなし。
	if _, _, err := app.sessions.CreateSession("wt-session", "0", 120, 40); err != nil {
		t.Fatal(err)
	}
	if err := app.sessions.SetRootPath("wt-session", `C:\Projects\myapp`); err != nil {
		t.Fatal(err)
	}
	if err := app.sessions.SetWorktreeInfo("wt-session", &tmux.SessionWorktreeInfo{
		Path:       `C:\Projects\myapp.wt\feature`,
		RepoPath:   `C:\Projects\myapp`,
		BranchName: "feature",
		BaseBranch: "main",
		IsDetached: false,
	}); err != nil {
		t.Fatal(err)
	}

	if got := app.CheckDirectoryConflict(`C:\Projects\myapp`); got != "" {
		t.Errorf("worktree session should not cause conflict, got %q", got)
	}

	// 通常セッションを追加 → コンフリクトあり。
	if _, _, err := app.sessions.CreateSession("normal", "0", 120, 40); err != nil {
		t.Fatal(err)
	}
	if err := app.sessions.SetRootPath("normal", `C:\Projects\myapp`); err != nil {
		t.Fatal(err)
	}

	if got := app.CheckDirectoryConflict(`C:\Projects\myapp`); got != "normal" {
		t.Errorf("expected conflict with normal, got %q", got)
	}
}

func TestCheckDirectoryConflictMixedSessions(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()

	// 通常セッション (dir=alpha)
	if _, _, err := app.sessions.CreateSession("normal-a", "0", 120, 40); err != nil {
		t.Fatal(err)
	}
	if err := app.sessions.SetRootPath("normal-a", `C:\Projects\alpha`); err != nil {
		t.Fatal(err)
	}

	// Worktreeセッション (repo=alpha, wt=alpha.wt/feat)
	if _, _, err := app.sessions.CreateSession("wt-a", "0", 120, 40); err != nil {
		t.Fatal(err)
	}
	if err := app.sessions.SetRootPath("wt-a", `C:\Projects\alpha`); err != nil {
		t.Fatal(err)
	}
	if err := app.sessions.SetWorktreeInfo("wt-a", &tmux.SessionWorktreeInfo{
		Path:       `C:\Projects\alpha.wt\feat`,
		RepoPath:   `C:\Projects\alpha`,
		BranchName: "feat",
		BaseBranch: "main",
		IsDetached: false,
	}); err != nil {
		t.Fatal(err)
	}

	// alphaはnormal-aがコンフリクト（worktreeセッションはスキップ）
	if got := app.CheckDirectoryConflict(`C:\Projects\alpha`); got != "normal-a" {
		t.Errorf("expected conflict with normal-a, got %q", got)
	}

	// betaはコンフリクトなし
	if got := app.CheckDirectoryConflict(`C:\Projects\beta`); got != "" {
		t.Errorf("expected no conflict, got %q", got)
	}
}

func TestCheckDirectoryConflict(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()

	// No sessions → no conflict.
	got := app.CheckDirectoryConflict(`C:\Projects\myapp`)
	if got != "" {
		t.Errorf("expected empty conflict, got %q", got)
	}

	// Create a session with root path.
	if _, _, err := app.sessions.CreateSession("sess1", "0", 120, 40); err != nil {
		t.Fatal(err)
	}
	if err := app.sessions.SetRootPath("sess1", `C:\Projects\myapp`); err != nil {
		t.Fatal(err)
	}

	// Now the path should conflict.
	got = app.CheckDirectoryConflict(`C:\Projects\myapp`)
	if got != "sess1" {
		t.Errorf("expected conflict with sess1, got %q", got)
	}

	// Whitespace trimming.
	got = app.CheckDirectoryConflict(`  C:\Projects\myapp  `)
	if got != "sess1" {
		t.Errorf("expected whitespace-trimmed conflict with sess1, got %q", got)
	}

	// Different path → no conflict.
	got = app.CheckDirectoryConflict(`C:\Projects\other`)
	if got != "" {
		t.Errorf("expected no conflict for different path, got %q", got)
	}
}

func TestCheckDirectoryConflictEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		dir  string
		want string
	}{
		{
			name: "empty string returns no conflict",
			dir:  "",
			want: "",
		},
		{
			name: "whitespace-only returns no conflict",
			dir:  "   ",
			want: "",
		},
		{
			name: "relative path does not match absolute",
			dir:  `Projects\myapp`,
			want: "",
		},
		{
			name: "dot path does not match",
			dir:  ".",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp()
			app.sessions = tmux.NewSessionManager()

			if _, _, err := app.sessions.CreateSession("sess1", "0", 120, 40); err != nil {
				t.Fatal(err)
			}
			if err := app.sessions.SetRootPath("sess1", `C:\Projects\myapp`); err != nil {
				t.Fatal(err)
			}

			got := app.CheckDirectoryConflict(tt.dir)
			if got != tt.want {
				t.Errorf("CheckDirectoryConflict(%q) = %q, want %q", tt.dir, got, tt.want)
			}
		})
	}
}

func TestFindSessionByRootPathCaseInsensitive(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()

	if _, _, err := app.sessions.CreateSession("test-session", "0", 120, 40); err != nil {
		t.Fatal(err)
	}
	if err := app.sessions.SetRootPath("test-session", `C:\Projects\MyApp`); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		dir  string
		want string
	}{
		{"exact case", `C:\Projects\MyApp`, "test-session"},
		{"lower case", `c:\projects\myapp`, "test-session"},
		{"upper case", `C:\PROJECTS\MYAPP`, "test-session"},
		{"mixed case", `C:\projects\MYAPP`, "test-session"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := app.findSessionByRootPath(tt.dir)
			if got != tt.want {
				t.Errorf("findSessionByRootPath(%q) = %q, want %q", tt.dir, got, tt.want)
			}
		})
	}
}

func TestCheckDirectoryConflictMultipleSessions(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()

	// Create two sessions with different root paths.
	if _, _, err := app.sessions.CreateSession("sess-a", "0", 120, 40); err != nil {
		t.Fatal(err)
	}
	if err := app.sessions.SetRootPath("sess-a", `C:\Projects\alpha`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := app.sessions.CreateSession("sess-b", "0", 120, 40); err != nil {
		t.Fatal(err)
	}
	if err := app.sessions.SetRootPath("sess-b", `C:\Projects\beta`); err != nil {
		t.Fatal(err)
	}

	// Each path should conflict with its own session.
	if got := app.CheckDirectoryConflict(`C:\Projects\alpha`); got != "sess-a" {
		t.Errorf("expected conflict with sess-a, got %q", got)
	}
	if got := app.CheckDirectoryConflict(`C:\Projects\beta`); got != "sess-b" {
		t.Errorf("expected conflict with sess-b, got %q", got)
	}
	// Unrelated path → no conflict.
	if got := app.CheckDirectoryConflict(`C:\Projects\gamma`); got != "" {
		t.Errorf("expected no conflict, got %q", got)
	}
}

func TestSetRootPath(t *testing.T) {
	sm := tmux.NewSessionManager()

	// Non-existent session returns error.
	if err := sm.SetRootPath("nonexistent", `C:\foo`); err == nil {
		t.Error("expected error for nonexistent session")
	}

	// Create session and set root path.
	if _, _, err := sm.CreateSession("test", "0", 120, 40); err != nil {
		t.Fatal(err)
	}
	if err := sm.SetRootPath("test", `C:\Projects\myapp`); err != nil {
		t.Fatalf("SetRootPath failed: %v", err)
	}

	// Verify via Snapshot.
	snapshots := sm.Snapshot()
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snapshots))
	}
	if snapshots[0].RootPath != `C:\Projects\myapp` {
		t.Errorf("RootPath = %q, want %q", snapshots[0].RootPath, `C:\Projects\myapp`)
	}
}

func TestStoreRootPath(t *testing.T) {
	t.Run("returns error when session manager is unavailable", func(t *testing.T) {
		app := NewApp()
		app.sessions = nil

		if err := app.storeRootPath("session-a", `C:\Projects\repo`); err == nil {
			t.Fatal("storeRootPath() expected session manager availability error")
		}
	})

	t.Run("returns error when session does not exist", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()

		if err := app.storeRootPath("missing-session", `C:\Projects\repo`); err == nil {
			t.Fatal("storeRootPath() expected missing-session error")
		}
	})

	t.Run("stores root path for existing session", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		if _, _, err := app.sessions.CreateSession("session-a", "0", 120, 40); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}

		if err := app.storeRootPath("session-a", `C:\Projects\repo`); err != nil {
			t.Fatalf("storeRootPath() error = %v", err)
		}

		snapshots := app.sessions.Snapshot()
		if len(snapshots) != 1 {
			t.Fatalf("snapshot count = %d, want 1", len(snapshots))
		}
		if snapshots[0].RootPath != `C:\Projects\repo` {
			t.Fatalf("RootPath = %q, want %q", snapshots[0].RootPath, `C:\Projects\repo`)
		}
	})
}

func TestCreateSessionEmitsSnapshot(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})

	events := make([]string, 0, 4)
	runtimeEventsEmitFn = func(_ context.Context, name string, _ ...interface{}) {
		events = append(events, name)
	}

	if _, err := app.CreateSession(os.TempDir(), "session-a", false); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	foundSnapshotEvent := false
	for _, name := range events {
		if name == "tmux:snapshot" || name == "tmux:snapshot-delta" {
			foundSnapshotEvent = true
			break
		}
	}
	if !foundSnapshotEvent {
		t.Fatalf("CreateSession() events = %v, want snapshot event", events)
	}
}

func TestKillSessionEmitsSnapshot(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})

	if _, _, err := app.sessions.CreateSession("session-a", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	events := make([]string, 0, 4)
	runtimeEventsEmitFn = func(_ context.Context, name string, _ ...interface{}) {
		events = append(events, name)
	}

	if err := app.KillSession("session-a", false); err != nil {
		t.Fatalf("KillSession() error = %v", err)
	}

	foundSnapshotEvent := false
	for _, name := range events {
		if name == "tmux:snapshot" || name == "tmux:snapshot-delta" {
			foundSnapshotEvent = true
			break
		}
	}
	if !foundSnapshotEvent {
		t.Fatalf("KillSession() events = %v, want snapshot event", events)
	}
}

func TestKillSessionClearsActiveSessionWhenTargetIsActive(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})

	if _, _, err := app.sessions.CreateSession("session-a", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	app.setActiveSessionName("session-a")

	if err := app.KillSession("session-a", false); err != nil {
		t.Fatalf("KillSession() error = %v", err)
	}
	if got := app.getActiveSessionName(); got != "" {
		t.Fatalf("active session = %q, want empty after kill", got)
	}
}

func waitForAsyncError(t *testing.T, done <-chan error, fallbackTimeout time.Duration, timeoutMessage string) error {
	t.Helper()
	timeout := fallbackTimeout
	if deadline, ok := t.Deadline(); ok {
		remaining := time.Until(deadline) / 4
		if remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-done:
		return err
	case <-timer.C:
		t.Fatalf("%s", timeoutMessage)
		return nil
	}
}

func waitForSignal(t *testing.T, done <-chan struct{}, fallbackTimeout time.Duration, timeoutMessage string) {
	t.Helper()
	timeout := fallbackTimeout
	if deadline, ok := t.Deadline(); ok {
		remaining := time.Until(deadline) / 4
		if remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		t.Fatalf("%s", timeoutMessage)
	}
}

func TestKillSessionStopsOutputBuffersOutsideOutputLock(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})

	_, pane, err := app.sessions.CreateSession("session-a", "0", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	paneID := fmt.Sprintf("%%%d", pane.ID)

	callbackRan := make(chan struct{}, 1)
	flusher := terminal.NewOutputFlushManager(16*time.Millisecond, 1024, func(_ string, _ []byte) {
		app.outputMu.Lock()
		app.outputMu.Unlock()
		select {
		case callbackRan <- struct{}{}:
		default:
		}
	})
	flusher.Start()
	flusher.Write(paneID, []byte("pending"))
	app.outputFlusher = flusher

	done := make(chan error, 1)
	go func() {
		done <- app.KillSession("session-a", false)
	}()

	killErr := waitForAsyncError(
		t,
		done,
		2*time.Second,
		"KillSession() timed out; possible outputMu -> Stop callback deadlock",
	)
	if killErr != nil {
		t.Fatalf("KillSession() error = %v", killErr)
	}

	waitForSignal(
		t,
		callbackRan,
		500*time.Millisecond,
		"output buffer callback did not run during KillSession()",
	)
}

func TestKillSessionWorktreeLookupFailureEmitsCleanupFailureEvent(t *testing.T) {
	origExecute := executeRouterRequestFn
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		executeRouterRequestFn = origExecute
		runtimeEventsEmitFn = origEmit
	})

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})

	executeRouterRequestFn = func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
		if req.Command != "kill-session" {
			return ipc.TmuxResponse{ExitCode: 1, Stderr: "unexpected command"}
		}
		return ipc.TmuxResponse{ExitCode: 0}
	}

	var cleanupPayload map[string]any
	runtimeEventsEmitFn = func(_ context.Context, name string, data ...interface{}) {
		if name != "worktree:cleanup-failed" || len(data) == 0 {
			return
		}
		payload, ok := data[0].(map[string]any)
		if ok {
			cleanupPayload = payload
		}
	}

	if err := app.KillSession("missing-session", true); err != nil {
		t.Fatalf("KillSession() error = %v, want nil because session kill already succeeded", err)
	}
	if cleanupPayload == nil {
		t.Fatal("expected worktree:cleanup-failed event payload")
	}
	if got := cleanupPayload["sessionName"]; got != "missing-session" {
		t.Fatalf("cleanup payload sessionName = %v, want missing-session", got)
	}
}

func TestKillSessionWithoutDeleteWorktreeLogsMetadataLookupFailure(t *testing.T) {
	origExecute := executeRouterRequestFn
	t.Cleanup(func() {
		executeRouterRequestFn = origExecute
	})

	logBuf := testutil.CaptureLogBuffer(t, slog.LevelDebug)

	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})

	executeRouterRequestFn = func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
		if req.Command != "kill-session" {
			return ipc.TmuxResponse{ExitCode: 1, Stderr: "unexpected command"}
		}
		return ipc.TmuxResponse{ExitCode: 0}
	}

	if err := app.KillSession("missing-session", false); err != nil {
		t.Fatalf("KillSession() error = %v, want nil when tmux kill succeeds", err)
	}
	if !strings.Contains(logBuf.String(), "failed to resolve worktree metadata for killed session") {
		t.Fatalf("expected metadata lookup warning log, got logs: %q", logBuf.String())
	}
}

func TestKillSessionDeleteWorktreeWithoutMetadataLogsDebug(t *testing.T) {
	origExecute := executeRouterRequestFn
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		executeRouterRequestFn = origExecute
		runtimeEventsEmitFn = origEmit
	})

	logBuf := testutil.CaptureLogBuffer(t, slog.LevelDebug)

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
	if _, _, err := app.sessions.CreateSession("session-a", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	executeRouterRequestFn = func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
		if req.Command != "kill-session" {
			return ipc.TmuxResponse{ExitCode: 1, Stderr: "unexpected command"}
		}
		return ipc.TmuxResponse{ExitCode: 0}
	}

	cleanupFailedEvents := 0
	runtimeEventsEmitFn = func(_ context.Context, name string, _ ...interface{}) {
		if name == "worktree:cleanup-failed" {
			cleanupFailedEvents++
		}
	}

	if err := app.KillSession("session-a", true); err != nil {
		t.Fatalf("KillSession() error = %v", err)
	}
	if cleanupFailedEvents != 0 {
		t.Fatalf("cleanup failure events = %d, want 0", cleanupFailedEvents)
	}
	if !strings.Contains(logBuf.String(), "deleteWorktree requested but session has no worktree metadata") {
		t.Fatalf("expected debug log for missing worktree metadata, got logs: %q", logBuf.String())
	}
}

func TestKillSessionDeleteWorktreeDirtyWorktreeEmitsFailureAndKeepsWorktree(t *testing.T) {
	testutil.SkipIfNoGit(t)

	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	repoPath := testutil.CreateTempGitRepo(t)
	repo, err := gitpkg.Open(repoPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	wtPath := filepath.Join(t.TempDir(), "kill-dirty-worktree")
	if err := repo.CreateWorktree(wtPath, "feature/kill-dirty", "HEAD"); err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, "dirty.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.setConfigSnapshot(config.DefaultConfig())
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
	if _, _, err := app.sessions.CreateSession("session-a", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if err := app.sessions.SetWorktreeInfo("session-a", &tmux.SessionWorktreeInfo{
		Path:       wtPath,
		RepoPath:   repoPath,
		BranchName: "feature/kill-dirty",
		BaseBranch: "HEAD",
		IsDetached: false,
	}); err != nil {
		t.Fatalf("SetWorktreeInfo() error = %v", err)
	}

	var cleanupPayload map[string]any
	runtimeEventsEmitFn = func(_ context.Context, name string, data ...interface{}) {
		if name != "worktree:cleanup-failed" || len(data) == 0 {
			return
		}
		payload, ok := data[0].(map[string]any)
		if ok {
			cleanupPayload = payload
		}
	}

	if err := app.KillSession("session-a", true); err != nil {
		t.Fatalf("KillSession() error = %v", err)
	}
	if cleanupPayload == nil {
		t.Fatal("expected worktree:cleanup-failed event payload")
	}
	if got := cleanupPayload["sessionName"]; got != "session-a" {
		t.Fatalf("cleanup payload sessionName = %v, want session-a", got)
	}
	if errText, _ := cleanupPayload["error"].(string); !strings.Contains(errText, "uncommitted changes") {
		t.Fatalf("cleanup payload error = %q, want uncommitted changes", errText)
	}
	if _, statErr := os.Stat(wtPath); statErr != nil {
		t.Fatalf("dirty worktree should remain after cleanup rejection, stat error = %v", statErr)
	}
}

func TestEmitWorktreeCleanupFailureFallsBackToBackgroundContext(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	eventCount := 0
	var emitCtx context.Context
	runtimeEventsEmitFn = func(ctx context.Context, _ string, _ ...interface{}) {
		eventCount++
		emitCtx = ctx
	}

	app := NewApp()
	app.emitWorktreeCleanupFailure("session-a", `C:\wt\session-a`, errors.New("cleanup failed"))

	if eventCount != 1 {
		t.Fatalf("event count = %d, want 1 when app context is nil", eventCount)
	}
	if emitCtx == nil {
		t.Fatal("emit context is nil, want background fallback context")
	}
}

func TestRollbackCreatedSessionDirect(t *testing.T) {
	origExecute := executeRouterRequestFn
	t.Cleanup(func() {
		executeRouterRequestFn = origExecute
	})

	t.Run("successful kill-session request", func(t *testing.T) {
		app := NewApp()
		app.router = tmux.NewCommandRouter(tmux.NewSessionManager(), nil, tmux.RouterOptions{})

		var captured ipc.TmuxRequest
		executeRouterRequestFn = func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
			captured = req
			return ipc.TmuxResponse{ExitCode: 0}
		}

		if err := app.rollbackCreatedSession("session-a"); err != nil {
			t.Fatalf("rollbackCreatedSession() error = %v", err)
		}
		if captured.Command != "kill-session" {
			t.Fatalf("command = %q, want kill-session", captured.Command)
		}
		if target, _ := captured.Flags["-t"].(string); target != "session-a" {
			t.Fatalf("target = %q, want %q", target, "session-a")
		}
	})

	t.Run("router failure returns wrapped error", func(t *testing.T) {
		app := NewApp()
		app.router = tmux.NewCommandRouter(tmux.NewSessionManager(), nil, tmux.RouterOptions{})

		executeRouterRequestFn = func(_ *tmux.CommandRouter, _ ipc.TmuxRequest) ipc.TmuxResponse {
			return ipc.TmuxResponse{ExitCode: 1, Stderr: "boom\n"}
		}

		if err := app.rollbackCreatedSession("session-a"); err == nil {
			t.Fatal("rollbackCreatedSession() expected error")
		}
	})
}

func TestActivateCreatedSessionDirect(t *testing.T) {
	t.Run("returns created snapshot and updates active session", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		if _, _, err := app.sessions.CreateSession("session-a", "0", 120, 40); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}

		snapshot, err := app.activateCreatedSession("session-a")
		if err != nil {
			t.Fatalf("activateCreatedSession() error = %v", err)
		}
		if snapshot.Name != "session-a" {
			t.Fatalf("snapshot.Name = %q, want %q", snapshot.Name, "session-a")
		}
		if got := app.getActiveSessionName(); got != "session-a" {
			t.Fatalf("active session = %q, want %q", got, "session-a")
		}
	})

	t.Run("returns error when session is missing", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		if _, err := app.activateCreatedSession("missing"); err == nil {
			t.Fatal("activateCreatedSession() expected missing-session error")
		}
	})
}

func TestCleanupSessionWorktreeSuccessPaths(t *testing.T) {
	testutil.SkipIfNoGit(t)

	t.Run("clean worktree is removed", func(t *testing.T) {
		repoPath := testutil.CreateTempGitRepo(t)
		repo, err := gitpkg.Open(repoPath)
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		wtPath := filepath.Join(t.TempDir(), "worktree-clean")
		if err := repo.CreateWorktree(wtPath, "feature/cleanup-clean", "HEAD"); err != nil {
			t.Fatalf("CreateWorktree() error = %v", err)
		}

		app := NewApp()
		app.setConfigSnapshot(config.DefaultConfig())
		app.cleanupSessionWorktree(worktreeCleanupParams{
			SessionName: "session-a",
			WtPath:      wtPath,
			RepoPath:    repoPath,
			BranchName:  "feature/cleanup-clean",
		})

		if _, statErr := os.Stat(wtPath); !os.IsNotExist(statErr) {
			t.Fatalf("worktree should be removed, stat error = %v", statErr)
		}
		branches, branchErr := repo.ListBranches()
		if branchErr != nil {
			t.Fatalf("ListBranches() error = %v", branchErr)
		}
		for _, branch := range branches {
			if branch == "feature/cleanup-clean" {
				t.Fatalf("orphaned worktree branch %q should be deleted", branch)
			}
		}
	})

	t.Run("force cleanup removes dirty worktree", func(t *testing.T) {
		repoPath := testutil.CreateTempGitRepo(t)
		repo, err := gitpkg.Open(repoPath)
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		wtPath := filepath.Join(t.TempDir(), "worktree-dirty")
		if err := repo.CreateWorktree(wtPath, "feature/cleanup-dirty", "HEAD"); err != nil {
			t.Fatalf("CreateWorktree() error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(wtPath, "dirty.txt"), []byte("dirty"), 0o644); err != nil {
			t.Fatalf("write dirty file: %v", err)
		}

		app := NewApp()
		cfg := config.DefaultConfig()
		cfg.Worktree.ForceCleanup = true
		app.setConfigSnapshot(cfg)
		app.cleanupSessionWorktree(worktreeCleanupParams{
			SessionName: "session-a",
			WtPath:      wtPath,
			RepoPath:    repoPath,
			BranchName:  "feature/cleanup-dirty",
		})

		if _, statErr := os.Stat(wtPath); !os.IsNotExist(statErr) {
			t.Fatalf("dirty worktree should be force-removed, stat error = %v", statErr)
		}
		branches, branchErr := repo.ListBranches()
		if branchErr != nil {
			t.Fatalf("ListBranches() error = %v", branchErr)
		}
		for _, branch := range branches {
			if branch == "feature/cleanup-dirty" {
				t.Fatalf("orphaned worktree branch %q should be deleted", branch)
			}
		}
	})

	t.Run("empty branch name keeps cleanup non-fatal", func(t *testing.T) {
		repoPath := testutil.CreateTempGitRepo(t)
		repo, err := gitpkg.Open(repoPath)
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		wtPath := filepath.Join(t.TempDir(), "worktree-detached")
		if err := repo.CreateWorktreeDetached(wtPath, "HEAD"); err != nil {
			t.Fatalf("CreateWorktreeDetached() error = %v", err)
		}

		app := NewApp()
		app.setConfigSnapshot(config.DefaultConfig())
		app.cleanupSessionWorktree(worktreeCleanupParams{
			SessionName: "session-a",
			WtPath:      wtPath,
			RepoPath:    repoPath,
			BranchName:  "",
		})

		if _, statErr := os.Stat(wtPath); !os.IsNotExist(statErr) {
			t.Fatalf("detached worktree should be removed, stat error = %v", statErr)
		}
	})
}

func TestWorktreeCleanupParamsFieldCountGuard(t *testing.T) {
	const expectedFieldCount = 4
	if got := reflect.TypeOf(worktreeCleanupParams{}).NumField(); got != expectedFieldCount {
		t.Fatalf("worktreeCleanupParams field count = %d, want %d", got, expectedFieldCount)
	}
}

func TestCleanupSessionWorktreeSkipsWhenWorktreePathEmpty(t *testing.T) {
	logBuf := testutil.CaptureLogBuffer(t, slog.LevelDebug)

	app := NewApp()
	app.cleanupSessionWorktree(worktreeCleanupParams{
		SessionName: "session-a",
		WtPath:      "   ",
		RepoPath:    "ignored",
		BranchName:  "ignored",
	})

	if !strings.Contains(logBuf.String(), "skip worktree cleanup: worktree path is empty") {
		t.Fatalf("expected skip log for empty worktree path, got logs: %q", logBuf.String())
	}
}

func TestCleanupSessionWorktreeEmitsFailureWhenRepoPathEmpty(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	app := NewApp()
	app.setRuntimeContext(context.Background())

	var eventPayload map[string]any
	runtimeEventsEmitFn = func(_ context.Context, name string, data ...interface{}) {
		if name != "worktree:cleanup-failed" || len(data) == 0 {
			return
		}
		payload, ok := data[0].(map[string]any)
		if ok {
			eventPayload = payload
		}
	}

	app.cleanupSessionWorktree(worktreeCleanupParams{
		SessionName: "session-a",
		WtPath:      `C:\worktree\session-a`,
		RepoPath:    "   ",
		BranchName:  "feature/test",
	})

	if eventPayload == nil {
		t.Fatal("expected worktree:cleanup-failed event payload")
	}
	if got := eventPayload["sessionName"]; got != "session-a" {
		t.Fatalf("payload sessionName = %v, want session-a", got)
	}
	if got := eventPayload["path"]; got != `C:\worktree\session-a` {
		t.Fatalf("payload path = %v, want %q", got, `C:\worktree\session-a`)
	}
	errMsg, _ := eventPayload["error"].(string)
	if !strings.Contains(errMsg, "repository path is empty") {
		t.Fatalf("payload error = %q, want repository path message", errMsg)
	}
}

func TestCleanupOrphanedLocalWorktreeBranchHandlesNilRepository(t *testing.T) {
	logBuf := testutil.CaptureLogBuffer(t, slog.LevelDebug)

	app := NewApp()
	app.cleanupOrphanedLocalWorktreeBranch(nil, "feature/orphan")

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "skip orphaned branch cleanup: repository is nil") {
		t.Fatalf("expected nil repository debug log, got logs: %q", logOutput)
	}
	if !strings.Contains(logOutput, "feature/orphan") {
		t.Fatalf("expected log to include branch name, got logs: %q", logOutput)
	}
}

func TestCleanupOrphanedLocalWorktreeBranchSkipsWhenBranchNameEmpty(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoPath := testutil.CreateTempGitRepo(t)
	repo, err := gitpkg.Open(repoPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	logBuf := testutil.CaptureLogBuffer(t, slog.LevelDebug)

	app := NewApp()
	app.cleanupOrphanedLocalWorktreeBranch(repo, "   ")

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "skip orphaned branch cleanup: branch name is empty") {
		t.Fatalf("expected empty-branch debug log, got logs: %q", logOutput)
	}
}

func TestGetSessionEnvValidation(t *testing.T) {
	app := NewApp()
	app.sessions = nil

	if _, err := app.GetSessionEnv("   "); err == nil {
		t.Fatal("GetSessionEnv() expected empty name validation error")
	}

	if _, err := app.GetSessionEnv("session-a"); err == nil {
		t.Fatal("GetSessionEnv() expected session manager availability error")
	}
}

func TestCleanupOrphanedLocalWorktreeBranchLogsWarnOnCleanupError(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoPath := testutil.CreateTempGitRepo(t)
	repo, err := gitpkg.Open(repoPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	logBuf := testutil.CaptureLogBuffer(t, slog.LevelDebug)

	// Break the repository to force CleanupLocalBranchIfOrphaned to fail.
	if err := os.RemoveAll(filepath.Join(repoPath, ".git")); err != nil {
		t.Fatalf("failed to remove .git: %v", err)
	}

	app := NewApp()
	app.cleanupOrphanedLocalWorktreeBranch(repo, "feature/broken")

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "failed to clean up orphaned local branch") {
		t.Fatalf("expected warn log for cleanup error, got logs: %q", logOutput)
	}
	if !strings.Contains(logOutput, "feature/broken") {
		t.Fatalf("expected branch name in log, got logs: %q", logOutput)
	}
}
func TestCleanupSessionWorktreeEmitsFailureWhenRepoOpenFails(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.setConfigSnapshot(config.DefaultConfig())

	var eventPayload map[string]any
	runtimeEventsEmitFn = func(_ context.Context, name string, data ...interface{}) {
		if name != "worktree:cleanup-failed" || len(data) == 0 {
			return
		}
		payload, ok := data[0].(map[string]any)
		if ok {
			eventPayload = payload
		}
	}

	app.cleanupSessionWorktree(worktreeCleanupParams{
		SessionName: "session-a",
		WtPath:      filepath.Join(t.TempDir(), "worktree-open-fail"),
		RepoPath:    filepath.Join(t.TempDir(), "nonexistent-repo"),
		BranchName:  "feature/test",
	})

	if eventPayload == nil {
		t.Fatal("expected worktree:cleanup-failed event payload")
	}
	if got := eventPayload["sessionName"]; got != "session-a" {
		t.Fatalf("payload sessionName = %v, want session-a", got)
	}
}

// TestValidationIntegration tests git package validation functions used by worktree API.
func TestValidationIntegration(t *testing.T) {
	t.Run("ValidateBranchName rejects dangerous input", func(t *testing.T) {
		dangerous := []string{"", "../hack", "-flag", ".hidden", "a..b"}
		for _, name := range dangerous {
			if err := gitpkg.ValidateBranchName(name); err == nil {
				t.Errorf("ValidateBranchName(%q) should have returned error", name)
			}
		}
	})

	t.Run("ValidateBranchName accepts valid input", func(t *testing.T) {
		valid := []string{"main", "feature/auth", "fix-123", "v1.0"}
		for _, name := range valid {
			if err := gitpkg.ValidateBranchName(name); err != nil {
				t.Errorf("ValidateBranchName(%q) unexpected error: %v", name, err)
			}
		}
	})
}
