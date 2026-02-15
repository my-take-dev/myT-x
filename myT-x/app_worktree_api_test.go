package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"myT-x/internal/config"
	gitpkg "myT-x/internal/git"
	"myT-x/internal/ipc"
	"myT-x/internal/testutil"
	"myT-x/internal/tmux"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func runGitInDir(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestShellExecFlag(t *testing.T) {
	tests := []struct {
		name  string
		shell string
		want  string
	}{
		{"cmd.exe", "cmd.exe", "/c"},
		{"CMD.EXE uppercase", "CMD.EXE", "/c"},
		{"bash.exe", "bash.exe", "-c"},
		{"wsl.exe", "wsl.exe", "-c"},
		{"powershell.exe", "powershell.exe", "-Command"},
		{"pwsh.exe", "pwsh.exe", "-Command"},
		{"absolute cmd path", `C:\Windows\System32\cmd.exe`, "/c"},
		{"absolute powershell path", `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`, "-Command"},
		{"unknown shell", "zsh.exe", "-Command"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shellExecFlag(tt.shell)
			if got != tt.want {
				t.Errorf("shellExecFlag(%q) = %q, want %q", tt.shell, got, tt.want)
			}
		})
	}
}

func TestRunSetupScripts(t *testing.T) {
	origExecuteSetup := executeSetupCommandFn
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		executeSetupCommandFn = origExecuteSetup
		runtimeEventsEmitFn = origEmit
	})

	t.Run("all scripts succeed and emit success event", func(t *testing.T) {
		app := NewApp()
		app.setRuntimeContext(context.Background())

		var ran []string
		executeSetupCommandFn = func(_ context.Context, _ string, _ string, script string, _ string) ([]byte, error) {
			ran = append(ran, script)
			return []byte("ok"), nil
		}

		var eventPayload map[string]any
		runtimeEventsEmitFn = func(_ context.Context, name string, data ...interface{}) {
			if name != "worktree:setup-complete" || len(data) == 0 {
				return
			}
			payload, ok := data[0].(map[string]any)
			if ok {
				eventPayload = payload
			}
		}

		app.runSetupScripts(t.TempDir(), "session-a", "powershell.exe", []string{"echo one", "echo two"})
		if len(ran) != 2 {
			t.Fatalf("executed scripts = %d, want 2", len(ran))
		}
		if eventPayload == nil {
			t.Fatal("expected worktree:setup-complete payload")
		}
		if success, _ := eventPayload["success"].(bool); !success {
			t.Fatalf("success payload = %v, want true", eventPayload["success"])
		}
	})

	t.Run("script failure stops sequence and emits failure event", func(t *testing.T) {
		app := NewApp()
		app.setRuntimeContext(context.Background())

		var ran []string
		executeSetupCommandFn = func(_ context.Context, _ string, _ string, script string, _ string) ([]byte, error) {
			ran = append(ran, script)
			if script == "bad-script" {
				return []byte("boom"), errors.New("exec failed")
			}
			return []byte("ok"), nil
		}

		var eventPayload map[string]any
		runtimeEventsEmitFn = func(_ context.Context, name string, data ...interface{}) {
			if name != "worktree:setup-complete" || len(data) == 0 {
				return
			}
			payload, ok := data[0].(map[string]any)
			if ok {
				eventPayload = payload
			}
		}

		app.runSetupScripts(t.TempDir(), "session-b", "powershell.exe", []string{"bad-script", "never-run"})
		if len(ran) != 1 {
			t.Fatalf("executed scripts = %d, want 1", len(ran))
		}
		if eventPayload == nil {
			t.Fatal("expected failure payload")
		}
		if success, _ := eventPayload["success"].(bool); success {
			t.Fatalf("success payload = %v, want false", eventPayload["success"])
		}
		errorText, _ := eventPayload["error"].(string)
		if !strings.Contains(errorText, "bad-script") {
			t.Fatalf("failure error = %q, want script name", errorText)
		}
	})

	t.Run("context deadline failure emits failure event", func(t *testing.T) {
		app := NewApp()
		app.setRuntimeContext(context.Background())

		executeSetupCommandFn = func(_ context.Context, _ string, _ string, _ string, _ string) ([]byte, error) {
			return nil, context.DeadlineExceeded
		}

		var eventPayload map[string]any
		runtimeEventsEmitFn = func(_ context.Context, name string, data ...interface{}) {
			if name != "worktree:setup-complete" || len(data) == 0 {
				return
			}
			payload, ok := data[0].(map[string]any)
			if ok {
				eventPayload = payload
			}
		}

		app.runSetupScripts(t.TempDir(), "session-c", "powershell.exe", []string{"slow-script"})
		if eventPayload == nil {
			t.Fatal("expected failure payload")
		}
		if success, _ := eventPayload["success"].(bool); success {
			t.Fatalf("success payload = %v, want false", eventPayload["success"])
		}
		errorText, _ := eventPayload["error"].(string)
		if !strings.Contains(errorText, "deadline exceeded") {
			t.Fatalf("failure error = %q, want deadline exceeded", errorText)
		}
	})

	t.Run("whitespace-only scripts are skipped", func(t *testing.T) {
		app := NewApp()
		app.setRuntimeContext(context.Background())

		var ran []string
		executeSetupCommandFn = func(_ context.Context, _ string, _ string, script string, _ string) ([]byte, error) {
			ran = append(ran, script)
			return []byte("ok"), nil
		}

		var eventPayload map[string]any
		runtimeEventsEmitFn = func(_ context.Context, name string, data ...interface{}) {
			if name != "worktree:setup-complete" || len(data) == 0 {
				return
			}
			payload, ok := data[0].(map[string]any)
			if ok {
				eventPayload = payload
			}
		}

		app.runSetupScripts(t.TempDir(), "session-d", "powershell.exe", []string{"echo one", "  ", "", "echo two"})
		if len(ran) != 2 {
			t.Fatalf("executed scripts = %d, want 2 (whitespace-only should be skipped)", len(ran))
		}
		if ran[0] != "echo one" || ran[1] != "echo two" {
			t.Fatalf("ran = %v, want [echo one, echo two]", ran)
		}
		if eventPayload == nil {
			t.Fatal("expected worktree:setup-complete payload")
		}
		if success, _ := eventPayload["success"].(bool); !success {
			t.Fatalf("success payload = %v, want true", eventPayload["success"])
		}
	})
}

func TestCopyConfigFilesToWorktree(t *testing.T) {
	t.Run("copies files successfully", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()

		// Create source file.
		if err := os.WriteFile(filepath.Join(repoDir, ".env"), []byte("KEY=val"), 0o644); err != nil {
			t.Fatal(err)
		}

		failures := copyConfigFilesToWorktree(repoDir, wtDir, []string{".env"})
		if len(failures) != 0 {
			t.Fatalf("unexpected failures: %v", failures)
		}

		// Verify destination file.
		data, err := os.ReadFile(filepath.Join(wtDir, ".env"))
		if err != nil {
			t.Fatalf("failed to read destination file: %v", err)
		}
		if string(data) != "KEY=val" {
			t.Errorf("destination file content = %q, want %q", string(data), "KEY=val")
		}
	})

	t.Run("skips missing source files", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()

		failures := copyConfigFilesToWorktree(repoDir, wtDir, []string{"nonexistent.env"})
		if len(failures) != 0 {
			t.Fatalf("missing files should be silently skipped, got failures: %v", failures)
		}
	})

	t.Run("rejects absolute paths", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()

		failures := copyConfigFilesToWorktree(repoDir, wtDir, []string{`C:\Windows\System32\config.sys`})
		if len(failures) != 0 {
			t.Fatalf("absolute paths should be skipped, not added to failures: %v", failures)
		}
		// Verify no file was created in wtDir.
		entries, _ := os.ReadDir(wtDir)
		if len(entries) != 0 {
			t.Errorf("expected empty wtDir, got %d entries", len(entries))
		}
	})

	t.Run("rejects path traversal with ..", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()

		// Create a file outside repoDir that path traversal would try to reach.
		outsideFile := filepath.Join(filepath.Dir(repoDir), "sensitive.txt")
		if err := os.WriteFile(outsideFile, []byte("secret"), 0o644); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(outsideFile)

		failures := copyConfigFilesToWorktree(repoDir, wtDir, []string{"../sensitive.txt"})
		if len(failures) != 0 {
			t.Fatalf("traversal paths should be skipped, not added to failures: %v", failures)
		}
	})

	t.Run("copies nested files with directory creation", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()

		// Create nested source file.
		nestedDir := filepath.Join(repoDir, "config")
		if err := os.MkdirAll(nestedDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(nestedDir, "app.yaml"), []byte("key: val"), 0o644); err != nil {
			t.Fatal(err)
		}

		failures := copyConfigFilesToWorktree(repoDir, wtDir, []string{filepath.Join("config", "app.yaml")})
		if len(failures) != 0 {
			t.Fatalf("unexpected failures: %v", failures)
		}

		data, err := os.ReadFile(filepath.Join(wtDir, "config", "app.yaml"))
		if err != nil {
			t.Fatalf("failed to read nested destination file: %v", err)
		}
		if string(data) != "key: val" {
			t.Errorf("nested file content = %q, want %q", string(data), "key: val")
		}
	})

	t.Run("empty file list", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()

		failures := copyConfigFilesToWorktree(repoDir, wtDir, []string{})
		if len(failures) != 0 {
			t.Fatalf("empty file list should produce no failures: %v", failures)
		}
	})

	t.Run("nil file list", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()

		failures := copyConfigFilesToWorktree(repoDir, wtDir, nil)
		if len(failures) != 0 {
			t.Fatalf("nil file list should produce no failures: %v", failures)
		}
	})
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
			got := app.findAvailableSessionName(tt.input)
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

		got := app.findAvailableSessionName("test")
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

			got := app.findSessionByWorktreePath(tt.wtPath)
			if got != tt.want {
				t.Errorf("findSessionByWorktreePath(%q) = %q, want %q", tt.wtPath, got, tt.want)
			}
		})
	}
}

func TestCheckWorktreePathConflict(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()

	// No sessions → no conflict.
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

func TestCreateSessionWithWorktreeValidation(t *testing.T) {
	t.Run("returns error when session manager is unavailable", func(t *testing.T) {
		app := NewApp()
		app.sessions = nil
		app.router = tmux.NewCommandRouter(nil, nil, tmux.RouterOptions{})
		app.setConfigSnapshot(config.DefaultConfig())

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
		app.setConfigSnapshot(config.DefaultConfig())

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
		app.setConfigSnapshot(cfg)

		if _, err := app.CreateSessionWithWorktree(t.TempDir(), "session-a", WorktreeSessionOptions{
			BranchName: "feature/test",
		}); err == nil {
			t.Fatal("CreateSessionWithWorktree() expected disabled feature error")
		}
	})

	t.Run("returns error when branch name is empty", func(t *testing.T) {
		repoPath := testutil.CreateTempGitRepo(t)
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
		app.setConfigSnapshot(config.DefaultConfig())

		if _, err := app.CreateSessionWithWorktree(repoPath, "session-a", WorktreeSessionOptions{}); err == nil {
			t.Fatal("CreateSessionWithWorktree() expected branch validation error")
		}
	})
}

func TestCreateSessionWithWorktreeSuccess(t *testing.T) {
	repoPath := testutil.CreateTempGitRepo(t)
	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
	app.setConfigSnapshot(config.DefaultConfig())

	originalExecuteRouterRequestFn := executeRouterRequestFn
	t.Cleanup(func() {
		executeRouterRequestFn = originalExecuteRouterRequestFn
	})

	executeRouterRequestFn = func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
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
	}

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
	if _, statErr := os.Stat(worktreePath); !os.IsNotExist(statErr) {
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
	if _, statErr := os.Stat(worktreePath); !os.IsNotExist(statErr) {
		t.Fatalf("worktree path should be removed, stat err = %v", statErr)
	}

	// The branch was pushed and should remain available.
	runGitInDir(t, repoPath, "rev-parse", "--verify", "refs/heads/feature/cleanup-pushed")

	branches, err := app.ListBranches(repoPath)
	if err != nil {
		t.Fatalf("ListBranches() error = %v", err)
	}
	found := false
	for _, branch := range branches {
		if branch == "feature/cleanup-pushed" {
			found = true
			break
		}
	}
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
	app.setConfigSnapshot(config.DefaultConfig())

	originalExecute := executeRouterRequestFn
	t.Cleanup(func() {
		executeRouterRequestFn = originalExecute
	})

	var requests []ipc.TmuxRequest
	executeRouterRequestFn = func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
		requests = append(requests, req)
		switch req.Command {
		case "new-session":
			return ipc.TmuxResponse{ExitCode: 0, Stdout: "existing-wt\n"}
		case "kill-session":
			return ipc.TmuxResponse{ExitCode: 1, Stderr: "simulated kill failure\n"}
		default:
			return ipc.TmuxResponse{ExitCode: 1, Stderr: "unexpected command\n"}
		}
	}

	originalLogger := slog.Default()
	var logBuf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() {
		slog.SetDefault(originalLogger)
	})

	_, err := app.CreateSessionWithExistingWorktree(repoPath, "existing-wt", repoPath, false)
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
	app.setConfigSnapshot(config.DefaultConfig())

	originalExecute := executeRouterRequestFn
	t.Cleanup(func() {
		executeRouterRequestFn = originalExecute
	})

	var requests []ipc.TmuxRequest
	executeRouterRequestFn = func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
		requests = append(requests, req)
		switch req.Command {
		case "new-session":
			return ipc.TmuxResponse{ExitCode: 0, Stdout: "existing-wt\n"}
		case "kill-session":
			return ipc.TmuxResponse{ExitCode: 0}
		default:
			return ipc.TmuxResponse{ExitCode: 1, Stderr: "unexpected command\n"}
		}
	}

	_, err := app.CreateSessionWithExistingWorktree(repoPath, "existing-wt", repoPath, false)
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

func TestCreateSessionWithExistingWorktreeTreatsBranchDetectionErrorAsDetached(t *testing.T) {
	repoPath := testutil.CreateTempGitRepo(t)

	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
	app.setConfigSnapshot(config.DefaultConfig())

	originalExecute := executeRouterRequestFn
	t.Cleanup(func() {
		executeRouterRequestFn = originalExecute
	})
	executeRouterRequestFn = func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
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
	}

	originalCurrentBranchFn := currentBranchFn
	t.Cleanup(func() {
		currentBranchFn = originalCurrentBranchFn
	})

	currentBranchFn = func(*gitpkg.Repository) (string, error) {
		return "", errors.New("simulated branch detection failure")
	}

	snapshot, err := app.CreateSessionWithExistingWorktree(repoPath, "existing-wt", repoPath, false)
	if err != nil {
		t.Fatalf("CreateSessionWithExistingWorktree() error = %v", err)
	}

	info, err := app.sessions.GetWorktreeInfo(snapshot.Name)
	if err != nil {
		t.Fatalf("GetWorktreeInfo() error = %v", err)
	}
	if info == nil {
		t.Fatal("GetWorktreeInfo() returned nil")
	}
	if !info.IsDetached {
		t.Fatal("expected IsDetached=true when branch detection fails with empty branch")
	}
}

func TestCreateSessionWithExistingWorktreeReturnsErrorWhenBranchDetectionFailsWithNonEmptyBranch(t *testing.T) {
	repoPath := testutil.CreateTempGitRepo(t)

	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
	app.setConfigSnapshot(config.DefaultConfig())

	originalExecute := executeRouterRequestFn
	t.Cleanup(func() {
		executeRouterRequestFn = originalExecute
	})
	executeRouterRequestFn = func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
		switch req.Command {
		case "new-session":
			sessionName, _ := req.Flags["-s"].(string)
			sessionName = strings.TrimSpace(sessionName)
			if _, _, err := app.sessions.CreateSession(sessionName, "0", 120, 40); err != nil {
				return ipc.TmuxResponse{ExitCode: 1, Stderr: err.Error() + "\n"}
			}
			return ipc.TmuxResponse{ExitCode: 0, Stdout: sessionName + "\n"}
		default:
			return ipc.TmuxResponse{ExitCode: 1, Stderr: "unexpected command\n"}
		}
	}

	originalCurrentBranchFn := currentBranchFn
	t.Cleanup(func() {
		currentBranchFn = originalCurrentBranchFn
	})

	// Return non-empty branch name WITH an error → should surface the error.
	currentBranchFn = func(*gitpkg.Repository) (string, error) {
		return "ambiguous-ref", errors.New("ambiguous ref detected")
	}

	_, err := app.CreateSessionWithExistingWorktree(repoPath, "existing-wt", repoPath, false)
	if err == nil {
		t.Fatal("CreateSessionWithExistingWorktree() expected error when branch detection returns non-empty branch with error")
	}
	if !strings.Contains(err.Error(), "failed to detect current branch") {
		t.Fatalf("error = %v, want 'failed to detect current branch'", err)
	}
}

func TestWorktreeStructFieldCounts(t *testing.T) {
	if got := reflect.TypeOf(WorktreeSessionOptions{}).NumField(); got != 4 {
		t.Fatalf("WorktreeSessionOptions field count = %d, want 4; update tests for new fields", got)
	}
	if got := reflect.TypeOf(WorktreeStatus{}).NumField(); got != 5 {
		t.Fatalf("WorktreeStatus field count = %d, want 5; update tests for new fields", got)
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
		for _, branch := range branches {
			if branch == target {
				return true
			}
		}
		return false
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
	if got := app.findSessionByWorktreePath(`C:\Projects\myapp.wt\feature`); got != "" {
		t.Fatalf("findSessionByWorktreePath() = %q, want empty when sessions is nil", got)
	}
}
