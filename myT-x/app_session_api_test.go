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
	"myT-x/internal/mcp"
	"myT-x/internal/terminal"
	"myT-x/internal/testutil"
	"myT-x/internal/tmux"
)

// NOTE: This file overrides package-level function variables
// (executeRouterRequestFn, runtimeEventsEmitFn). Do not use t.Parallel() here.
// Use stubExecuteRouterRequest() to stub executeRouterRequestFn with automatic restore.

// stubExecuteRouterRequest replaces executeRouterRequestFn with fn for the
// duration of the test and restores the original via t.Cleanup.
// Do NOT call t.Parallel() in tests that use this helper;
// package-level variable replacement is not concurrent-safe.
func stubExecuteRouterRequest(t *testing.T, fn func(*tmux.CommandRouter, ipc.TmuxRequest) ipc.TmuxResponse) {
	t.Helper()
	orig := executeRouterRequestFn
	t.Cleanup(func() { executeRouterRequestFn = orig })
	executeRouterRequestFn = fn
}

func stubNewSessionCommandSuccess(t *testing.T, app *App) {
	t.Helper()
	stubExecuteRouterRequest(t, func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
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
}

func TestCreateSessionOptionsFieldCountGuard(t *testing.T) {
	// Guard against silent divergence between CreateSessionOptions and
	// WorktreeSessionOptions (which mirrors these fields at app_worktree_api.go:153-157).
	// When adding a field here, also update:
	//   - WorktreeSessionOptions in app_worktree_api.go
	//   - the mapping in createSessionForDirectory / applySessionEnvFlags
	//   - frontend models.ts CreateSessionOptions class
	const expectedFieldCount = 3
	if got := reflect.TypeFor[CreateSessionOptions]().NumField(); got != expectedFieldCount {
		t.Fatalf("CreateSessionOptions field count = %d, want %d; "+
			"update WorktreeSessionOptions mapping, applySessionEnvFlags callers, and frontend models.ts",
			got, expectedFieldCount)
	}
}

func TestApplySessionEnvFlagsSetsSessionFlags(t *testing.T) {
	// Verify that applySessionEnvFlags correctly sets session-level flags,
	// covering the shared code path used by both CreateSession and
	// CreateSessionWithWorktree.
	tests := []struct {
		name             string
		useClaudeEnv     bool
		usePaneEnv       bool
		wantUseClaudeEnv *bool
		wantUsePaneEnv   *bool
	}{
		{
			name:             "both flags true",
			useClaudeEnv:     true,
			usePaneEnv:       true,
			wantUseClaudeEnv: testutil.Ptr(true),
			wantUsePaneEnv:   testutil.Ptr(true),
		},
		{
			name:             "both flags false",
			useClaudeEnv:     false,
			usePaneEnv:       false,
			wantUseClaudeEnv: nil,
			wantUsePaneEnv:   nil,
		},
		{
			name:             "only useClaudeEnv",
			useClaudeEnv:     true,
			usePaneEnv:       false,
			wantUseClaudeEnv: testutil.Ptr(true),
			wantUsePaneEnv:   nil,
		},
		{
			name:             "only usePaneEnv",
			useClaudeEnv:     false,
			usePaneEnv:       true,
			wantUseClaudeEnv: nil,
			wantUsePaneEnv:   testutil.Ptr(true),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions := tmux.NewSessionManager()
			if _, _, err := sessions.CreateSession("test-session", "0", 120, 40); err != nil {
				t.Fatalf("CreateSession() error = %v", err)
			}

			applySessionEnvFlags(sessions, "test-session", tt.useClaudeEnv, tt.usePaneEnv)

			session, ok := sessions.GetSession("test-session")
			if !ok {
				t.Fatal("expected session to exist after applySessionEnvFlags")
			}

			if tt.wantUseClaudeEnv == nil {
				if session.UseClaudeEnv != nil {
					t.Errorf("UseClaudeEnv = %v, want nil", session.UseClaudeEnv)
				}
			} else {
				if session.UseClaudeEnv == nil || *session.UseClaudeEnv != *tt.wantUseClaudeEnv {
					got := "<nil>"
					if session.UseClaudeEnv != nil {
						got = fmt.Sprintf("%v", *session.UseClaudeEnv)
					}
					t.Errorf("UseClaudeEnv = %s, want %v", got, *tt.wantUseClaudeEnv)
				}
			}

			if tt.wantUsePaneEnv == nil {
				if session.UsePaneEnv != nil {
					t.Errorf("UsePaneEnv = %v, want nil", session.UsePaneEnv)
				}
			} else {
				if session.UsePaneEnv == nil || *session.UsePaneEnv != *tt.wantUsePaneEnv {
					got := "<nil>"
					if session.UsePaneEnv != nil {
						got = fmt.Sprintf("%v", *session.UsePaneEnv)
					}
					t.Errorf("UsePaneEnv = %s, want %v", got, *tt.wantUsePaneEnv)
				}
			}
		})
	}
}

func TestCreateRenameKillSessionValidation(t *testing.T) {
	t.Run("CreateSession returns error when root path is empty", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
		if _, err := app.CreateSession("   ", "session-a", CreateSessionOptions{}); err == nil {
			t.Fatal("CreateSession() expected root path validation error")
		}
	})

	t.Run("CreateSession sanitizes empty session name to fallback", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
		stubNewSessionCommandSuccess(t, app)
		snapshot, err := app.CreateSession(t.TempDir(), "   ", CreateSessionOptions{})
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		if snapshot.Name != "session" {
			t.Fatalf("CreateSession() session name = %q, want %q (fallback)", snapshot.Name, "session")
		}
	})

	t.Run("CreateSession returns error when session manager is unavailable", func(t *testing.T) {
		app := NewApp()
		if _, err := app.CreateSession(t.TempDir(), "session-a", CreateSessionOptions{}); err == nil {
			t.Fatal("CreateSession() expected session manager availability error")
		}
	})

	t.Run("CreateSession returns error when router is unavailable", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		if _, err := app.CreateSession(t.TempDir(), "session-a", CreateSessionOptions{}); err == nil {
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
	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})

	stubExecuteRouterRequest(t, func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
		if req.Command != "new-session" {
			return ipc.TmuxResponse{ExitCode: 1, Stderr: "unexpected command"}
		}
		return ipc.TmuxResponse{ExitCode: 0, Stdout: "   "}
	})

	if _, err := app.CreateSession(t.TempDir(), "session-a", CreateSessionOptions{}); err == nil {
		t.Fatal("CreateSession() expected error when tmux returns empty created name")
	}
}

func TestCreateSessionRollsBackWhenStoreRootPathFails(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})

	var rollbackTargets []string
	stubExecuteRouterRequest(t, func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
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
	})

	_, err := app.CreateSession(t.TempDir(), "session-a", CreateSessionOptions{})
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

	stubExecuteRouterRequest(t, func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
		if req.Command != "new-session" {
			return ipc.TmuxResponse{ExitCode: 1, Stderr: "unexpected command"}
		}
		sessionName, _ := req.Flags["-s"].(string)
		if _, _, err := app.sessions.CreateSession(sessionName, "0", 120, 40); err != nil {
			return ipc.TmuxResponse{ExitCode: 1, Stderr: err.Error()}
		}
		return ipc.TmuxResponse{ExitCode: 0, Stdout: sessionName}
	})

	// Keep git-dir detection passing but force CurrentBranch() to fail.
	// This validates that metadata-enrichment failures are logged without
	// breaking session creation.
	headPath := filepath.Join(repoPath, ".git", "HEAD")
	if err := os.WriteFile(headPath, []byte("ref: refs/heads/does-not-exist\n"), 0o644); err != nil {
		t.Fatalf("failed to corrupt HEAD for metadata-failure test: %v", err)
	}

	if _, err := app.CreateSession(repoPath, "session-a", CreateSessionOptions{}); err != nil {
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

	var capturedReq ipc.TmuxRequest
	stubExecuteRouterRequest(t, func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
		capturedReq = req
		sessionName, _ := req.Flags["-s"].(string)
		if _, _, err := app.sessions.CreateSession(sessionName, "0", 120, 40); err != nil {
			return ipc.TmuxResponse{ExitCode: 1, Stderr: err.Error()}
		}
		return ipc.TmuxResponse{ExitCode: 0, Stdout: sessionName}
	})

	if _, err := app.CreateSession(t.TempDir(), "team-session", CreateSessionOptions{EnableAgentTeam: true}); err != nil {
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

func TestCreateSessionEnvFlags(t *testing.T) {
	tests := []struct {
		name             string
		useClaudeEnv     bool
		usePaneEnv       bool
		wantUseClaudeEnv *bool // nil = expect nil (Set not called), non-nil = expect pointer match
		wantUsePaneEnv   *bool
	}{
		{
			name:             "useClaudeEnv=true, usePaneEnv=false",
			useClaudeEnv:     true,
			usePaneEnv:       false,
			wantUseClaudeEnv: testutil.Ptr(true),
			wantUsePaneEnv:   nil, // false -> Set not called -> remains nil
		},
		{
			name:             "useClaudeEnv=false, usePaneEnv=true",
			useClaudeEnv:     false,
			usePaneEnv:       true,
			wantUseClaudeEnv: nil, // false -> Set not called -> remains nil
			wantUsePaneEnv:   testutil.Ptr(true),
		},
		{
			name:             "useClaudeEnv=true, usePaneEnv=true",
			useClaudeEnv:     true,
			usePaneEnv:       true,
			wantUseClaudeEnv: testutil.Ptr(true),
			wantUsePaneEnv:   testutil.Ptr(true),
		},
		{
			name:             "useClaudeEnv=false, usePaneEnv=false",
			useClaudeEnv:     false,
			usePaneEnv:       false,
			wantUseClaudeEnv: nil, // false -> Set not called -> remains nil (legacy path)
			wantUsePaneEnv:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp()
			app.sessions = tmux.NewSessionManager()
			app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})

			stubExecuteRouterRequest(t, func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
				sessionName, _ := req.Flags["-s"].(string)
				if _, _, err := app.sessions.CreateSession(sessionName, "0", 120, 40); err != nil {
					return ipc.TmuxResponse{ExitCode: 1, Stderr: err.Error()}
				}
				return ipc.TmuxResponse{ExitCode: 0, Stdout: sessionName}
			})

			sessionName := "test-session"
			if _, err := app.CreateSession(t.TempDir(), sessionName, CreateSessionOptions{UseClaudeEnv: tt.useClaudeEnv, UsePaneEnv: tt.usePaneEnv}); err != nil {
				t.Fatalf("CreateSession() error = %v", err)
			}

			session, ok := app.sessions.GetSession(sessionName)
			if !ok {
				t.Fatalf("expected session %q to exist after creation", sessionName)
			}

			// Verify *bool pointer state to distinguish nil (Set not called) from *false.
			if tt.wantUseClaudeEnv == nil {
				if session.UseClaudeEnv != nil {
					t.Errorf("UseClaudeEnv = %v, want nil (Set should not be called for false)", *session.UseClaudeEnv)
				}
			} else {
				if session.UseClaudeEnv == nil || *session.UseClaudeEnv != *tt.wantUseClaudeEnv {
					got := "<nil>"
					if session.UseClaudeEnv != nil {
						got = fmt.Sprintf("%v", *session.UseClaudeEnv)
					}
					t.Errorf("UseClaudeEnv = %s, want %v", got, *tt.wantUseClaudeEnv)
				}
			}

			if tt.wantUsePaneEnv == nil {
				if session.UsePaneEnv != nil {
					t.Errorf("UsePaneEnv = %v, want nil (Set should not be called for false)", *session.UsePaneEnv)
				}
			} else {
				if session.UsePaneEnv == nil || *session.UsePaneEnv != *tt.wantUsePaneEnv {
					got := "<nil>"
					if session.UsePaneEnv != nil {
						got = fmt.Sprintf("%v", *session.UsePaneEnv)
					}
					t.Errorf("UsePaneEnv = %s, want %v", got, *tt.wantUsePaneEnv)
				}
			}
		})
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
	runtimeEventsEmitFn = func(_ context.Context, name string, _ ...any) {
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
	foundSessionRenamedEvent := false
	for _, name := range events {
		if name == "tmux:session-renamed" {
			foundSessionRenamedEvent = true
		}
		if name == "tmux:snapshot" || name == "tmux:snapshot-delta" {
			foundSnapshotEvent = true
		}
	}
	if !foundSessionRenamedEvent {
		t.Fatalf("RenameSession() events = %v, want tmux:session-renamed event", events)
	}
	if !foundSnapshotEvent {
		t.Fatalf("RenameSession() events = %v, want snapshot event", events)
	}
}

func TestRenameSessionCleansUpMCPStateForOldSessionName(t *testing.T) {
	registry := mcp.NewRegistry()
	if err := registry.Register(mcp.Definition{
		ID:      "memory",
		Name:    "Memory",
		Command: "memory-cmd",
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	app.mcpManager = mcp.NewManager(registry, func(string, any) {})
	if _, _, err := app.sessions.CreateSession("old-name", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if err := app.mcpManager.SetEnabled("old-name", "memory", true); err != nil {
		t.Fatalf("SetEnabled() error = %v", err)
	}

	if err := app.RenameSession("old-name", "new-name"); err != nil {
		t.Fatalf("RenameSession() error = %v", err)
	}

	oldSnapshots, err := app.mcpManager.SnapshotForSession("old-name")
	if err != nil {
		t.Fatalf("SnapshotForSession(old-name) error = %v", err)
	}
	if len(oldSnapshots) != 1 {
		t.Fatalf("SnapshotForSession(old-name) length = %d, want 1", len(oldSnapshots))
	}
	if oldSnapshots[0].Enabled {
		t.Fatal("old session MCP state still enabled after rename")
	}

	newSnapshots, err := app.mcpManager.SnapshotForSession("new-name")
	if err != nil {
		t.Fatalf("SnapshotForSession(new-name) error = %v", err)
	}
	if len(newSnapshots) != 1 {
		t.Fatalf("SnapshotForSession(new-name) length = %d, want 1", len(newSnapshots))
	}
	if newSnapshots[0].Enabled {
		t.Fatal("new session MCP state should start from default disabled state")
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

func TestQuickStartSession(t *testing.T) {
	t.Run("creates a new session from configured default directory", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
		stubNewSessionCommandSuccess(t, app)

		rootDir := filepath.Clean(filepath.Join(t.TempDir(), "quick-start-project"))
		if err := os.MkdirAll(rootDir, 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
		wantRoot := rootDir
		if resolvedRoot, err := filepath.EvalSymlinks(rootDir); err == nil {
			wantRoot = resolvedRoot
		}

		cfg := config.DefaultConfig()
		cfg.DefaultSessionDir = rootDir
		app.setConfigSnapshot(cfg)

		snapshot, err := app.QuickStartSession()
		if err != nil {
			t.Fatalf("QuickStartSession() error = %v", err)
		}
		if snapshot.RootPath != wantRoot {
			t.Fatalf("RootPath = %q, want %q", snapshot.RootPath, wantRoot)
		}
		if got := app.getActiveSessionName(); got != snapshot.Name {
			t.Fatalf("active session = %q, want %q", got, snapshot.Name)
		}
	})

	t.Run("falls back to launch directory when default is empty", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
		stubNewSessionCommandSuccess(t, app)

		launchDir := filepath.Clean(filepath.Join(t.TempDir(), "launch-root"))
		if err := os.MkdirAll(launchDir, 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
		wantRoot := launchDir
		if resolvedRoot, err := filepath.EvalSymlinks(launchDir); err == nil {
			wantRoot = resolvedRoot
		}
		app.launchDir = launchDir
		app.setConfigSnapshot(config.DefaultConfig())

		snapshot, err := app.QuickStartSession()
		if err != nil {
			t.Fatalf("QuickStartSession() error = %v", err)
		}
		if snapshot.RootPath != wantRoot {
			t.Fatalf("RootPath = %q, want %q", snapshot.RootPath, wantRoot)
		}
	})

	t.Run("reuses existing session when root path conflicts", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
		stubNewSessionCommandSuccess(t, app)

		rootDir := filepath.Clean(filepath.Join(t.TempDir(), "existing-root"))
		if err := os.MkdirAll(rootDir, 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
		wantRoot := rootDir
		if resolvedRoot, err := filepath.EvalSymlinks(rootDir); err == nil {
			wantRoot = resolvedRoot
		}
		if _, _, err := app.sessions.CreateSession("existing-session", "0", 120, 40); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		if err := app.sessions.SetRootPath("existing-session", wantRoot); err != nil {
			t.Fatalf("SetRootPath() error = %v", err)
		}

		cfg := config.DefaultConfig()
		cfg.DefaultSessionDir = rootDir
		app.setConfigSnapshot(cfg)

		snapshot, err := app.QuickStartSession()
		if err != nil {
			t.Fatalf("QuickStartSession() error = %v", err)
		}
		if snapshot.Name != "existing-session" {
			t.Fatalf("snapshot.Name = %q, want %q", snapshot.Name, "existing-session")
		}
		if snapshot.RootPath != wantRoot {
			t.Fatalf("RootPath = %q, want %q", snapshot.RootPath, wantRoot)
		}
		if got := app.getActiveSessionName(); got != "existing-session" {
			t.Fatalf("active session = %q, want %q", got, "existing-session")
		}
		if got := len(app.sessions.Snapshot()); got != 1 {
			t.Fatalf("session count = %d, want 1", got)
		}
	})

	t.Run("creates missing default directory before creating session", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
		stubNewSessionCommandSuccess(t, app)

		missingDir := filepath.Clean(filepath.Join(t.TempDir(), "missing", "project"))
		if _, err := os.Stat(missingDir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("precondition failed: expected missing directory, got err=%v", err)
		}

		cfg := config.DefaultConfig()
		cfg.DefaultSessionDir = missingDir
		app.setConfigSnapshot(cfg)

		snapshot, err := app.QuickStartSession()
		if err != nil {
			t.Fatalf("QuickStartSession() error = %v", err)
		}
		if snapshot.RootPath != missingDir {
			t.Fatalf("RootPath = %q, want %q", snapshot.RootPath, missingDir)
		}
		info, statErr := os.Stat(missingDir)
		if statErr != nil {
			t.Fatalf("Stat() error = %v", statErr)
		}
		if !info.IsDir() {
			t.Fatalf("expected %q to be directory", missingDir)
		}
	})
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

	// Worktreeセッションのみ -> コンフリクトなし。
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

	// 通常セッションを追加 -> コンフリクトあり。
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

	// No sessions -> no conflict.
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

	// Different path -> no conflict.
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
	// Unrelated path -> no conflict.
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
	runtimeEventsEmitFn = func(_ context.Context, name string, _ ...any) {
		events = append(events, name)
	}

	if _, err := app.CreateSession(os.TempDir(), "session-a", CreateSessionOptions{}); err != nil {
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
	runtimeEventsEmitFn = func(_ context.Context, name string, _ ...any) {
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

func TestKillSession_NilMCPManager(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
	app.mcpManager = nil

	if _, _, err := app.sessions.CreateSession("session-a", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	stubExecuteRouterRequest(t, func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
		if req.Command != "kill-session" {
			return ipc.TmuxResponse{ExitCode: 1, Stderr: "unexpected command"}
		}
		return ipc.TmuxResponse{ExitCode: 0}
	})

	if err := app.KillSession("session-a", false); err != nil {
		t.Fatalf("KillSession() error with nil mcpManager = %v", err)
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
	runtimeEventsEmitFn = func(_ context.Context, name string, data ...any) {
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
	runtimeEventsEmitFn = func(_ context.Context, name string, _ ...any) {
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
	runtimeEventsEmitFn = func(_ context.Context, name string, data ...any) {
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

// TestEmitWorktreeCleanupFailureDropsEventWhenContextIsNil verifies that
// emitWorktreeCleanupFailure silently drops the event when the Wails runtime
// context is nil (I-14: no context.Background() fallback).
func TestEmitWorktreeCleanupFailureDropsEventWhenContextIsNil(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	eventCount := 0
	runtimeEventsEmitFn = func(_ context.Context, _ string, _ ...any) {
		eventCount++
	}

	app := NewApp()
	// No setRuntimeContext call — context is nil.
	app.emitWorktreeCleanupFailure("session-a", `C:\wt\session-a`, errors.New("cleanup failed"))

	if eventCount != 0 {
		t.Fatalf("event count = %d, want 0: event must be dropped when runtime context is nil", eventCount)
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

		if _, statErr := os.Stat(wtPath); !errors.Is(statErr, os.ErrNotExist) {
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

		if _, statErr := os.Stat(wtPath); !errors.Is(statErr, os.ErrNotExist) {
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

		if _, statErr := os.Stat(wtPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("detached worktree should be removed, stat error = %v", statErr)
		}
	})
}

func TestWorktreeCleanupParamsFieldCountGuard(t *testing.T) {
	const expectedFieldCount = 4
	if got := reflect.TypeFor[worktreeCleanupParams]().NumField(); got != expectedFieldCount {
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
	runtimeEventsEmitFn = func(_ context.Context, name string, data ...any) {
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
	runtimeEventsEmitFn = func(_ context.Context, name string, data ...any) {
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

// TestCreateSessionForDirectoryFillOnlyPriority tests the fill-only merge semantics
// of createSessionForDirectory when useClaudeEnv=true and enableAgentTeam=true.
//
// NOTE: テスト対象の createSessionForDirectory は app_worktree_api.go に定義。
// 本テストは CreateSession 経由の統合テストであるため app_session_api_test.go に配置。
//
// Fill-only rule: agent_team env vars take priority; claude_env fills only missing keys.
func TestCreateSessionForDirectoryFillOnlyPriority(t *testing.T) {
	tests := []struct {
		name            string
		enableAgentTeam bool
		useClaudeEnv    bool
		claudeEnvVars   map[string]string
		verifyEnv       func(t *testing.T, capturedEnv map[string]string)
	}{
		{
			name:            "agent team + claude env: agent team keys take priority over claude_env (fill-only)",
			enableAgentTeam: true,
			useClaudeEnv:    true,
			claudeEnvVars: map[string]string{
				"CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "should_be_ignored",
				"CLAUDE_CODE_TEAM_NAME":                "should_be_ignored",
				"CLAUDE_EXTRA":                         "extra_value",
			},
			verifyEnv: func(t *testing.T, capturedEnv map[string]string) {
				t.Helper()
				// Agent team vars must retain their original values (not overwritten by claude_env).
				agentVars := agentTeamEnvVars("test-session")
				for key, wantValue := range agentVars {
					if got := capturedEnv[key]; got != wantValue {
						t.Errorf("env[%q] = %q, want %q (agent team env must take priority)", key, got, wantValue)
					}
				}
				// Claude env keys that don't conflict with agent team vars must be filled.
				if got := capturedEnv["CLAUDE_EXTRA"]; got != "extra_value" {
					t.Errorf("env[\"CLAUDE_EXTRA\"] = %q, want %q (claude_env fill-only for non-conflicting key)", got, "extra_value")
				}
			},
		},
		{
			name:            "claude env only (no agent team): all claude_env vars applied",
			enableAgentTeam: false,
			useClaudeEnv:    true,
			claudeEnvVars: map[string]string{
				"CLAUDE_CODE_EFFORT_LEVEL": "high",
				"CUSTOM_KEY":               "custom_value",
			},
			verifyEnv: func(t *testing.T, capturedEnv map[string]string) {
				t.Helper()
				if got := capturedEnv["CLAUDE_CODE_EFFORT_LEVEL"]; got != "high" {
					t.Errorf("env[\"CLAUDE_CODE_EFFORT_LEVEL\"] = %q, want %q", got, "high")
				}
				if got := capturedEnv["CUSTOM_KEY"]; got != "custom_value" {
					t.Errorf("env[\"CUSTOM_KEY\"] = %q, want %q", got, "custom_value")
				}
			},
		},
		{
			name:            "agent team only (no claude env): only agent team vars present",
			enableAgentTeam: true,
			useClaudeEnv:    false,
			claudeEnvVars: map[string]string{
				"CLAUDE_EXTRA": "should_not_appear",
			},
			verifyEnv: func(t *testing.T, capturedEnv map[string]string) {
				t.Helper()
				agentVars := agentTeamEnvVars("test-session")
				for key, wantValue := range agentVars {
					if got := capturedEnv[key]; got != wantValue {
						t.Errorf("env[%q] = %q, want %q", key, got, wantValue)
					}
				}
				if _, ok := capturedEnv["CLAUDE_EXTRA"]; ok {
					t.Error("env[\"CLAUDE_EXTRA\"] should not be present when useClaudeEnv=false")
				}
			},
		},
		{
			name:            "neither agent team nor claude env: env map is empty or nil",
			enableAgentTeam: false,
			useClaudeEnv:    false,
			claudeEnvVars: map[string]string{
				"CLAUDE_EXTRA": "should_not_appear",
			},
			verifyEnv: func(t *testing.T, capturedEnv map[string]string) {
				t.Helper()
				if len(capturedEnv) != 0 {
					t.Errorf("env should be empty when both flags are false, got %v", capturedEnv)
				}
			},
		},
		{
			name:            "claude env enabled but config has nil ClaudeEnv: no vars added",
			enableAgentTeam: false,
			useClaudeEnv:    true,
			claudeEnvVars:   nil, // nil signals that ClaudeEnv config is nil
			verifyEnv: func(t *testing.T, capturedEnv map[string]string) {
				t.Helper()
				if len(capturedEnv) != 0 {
					t.Errorf("env should be empty when ClaudeEnv config is nil, got %v", capturedEnv)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp()
			app.sessions = tmux.NewSessionManager()
			app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})

			// Configure claude_env in the app config snapshot.
			cfg := config.DefaultConfig()
			if tt.claudeEnvVars != nil {
				cfg.ClaudeEnv = &config.ClaudeEnvConfig{Vars: tt.claudeEnvVars}
			} else {
				cfg.ClaudeEnv = nil
			}
			app.setConfigSnapshot(cfg)

			var capturedEnv map[string]string
			stubExecuteRouterRequest(t, func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
				capturedEnv = req.Env
				sessionName, _ := req.Flags["-s"].(string)
				if _, _, err := app.sessions.CreateSession(sessionName, "0", 120, 40); err != nil {
					return ipc.TmuxResponse{ExitCode: 1, Stderr: err.Error()}
				}
				return ipc.TmuxResponse{ExitCode: 0, Stdout: sessionName}
			})

			opts := CreateSessionOptions{
				EnableAgentTeam: tt.enableAgentTeam,
				UseClaudeEnv:    tt.useClaudeEnv,
			}
			if _, err := app.CreateSession(t.TempDir(), "test-session", opts); err != nil {
				t.Fatalf("CreateSession() error = %v", err)
			}

			if capturedEnv == nil {
				capturedEnv = map[string]string{}
			}
			tt.verifyEnv(t, capturedEnv)
		})
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

func TestSanitizeSessionName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "plain name unchanged", input: "my-session", want: "my-session"},
		{name: "dot replaced", input: "my.session", want: "my-session"},
		{name: "colon replaced", input: "D:", want: "D"},
		{name: "multiple dots", input: "a.b.c", want: "a-b-c"},
		{name: "dot and colon mixed", input: "host:8080.local", want: "host-8080-local"},
		{name: "consecutive invalid chars collapsed", input: "a.:b", want: "a-b"},
		{name: "leading dot trimmed", input: ".hidden", want: "hidden"},
		{name: "trailing dot trimmed", input: "name.", want: "name"},
		{name: "all invalid returns fallback", input: ".:", want: "quick-session"},
		{name: "empty returns fallback", input: "", want: "quick-session"},
		{name: "already clean", input: "clean-name-123", want: "clean-name-123"},
		{name: "single colon only", input: ":", want: "quick-session"},
		{name: "single dot only", input: ".", want: "quick-session"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeSessionName(tt.input, "quick-session")
			if got != tt.want {
				t.Errorf("sanitizeSessionName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
