package worktree

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"myT-x/internal/config"
	gitpkg "myT-x/internal/git"
	"myT-x/internal/testutil"
	"myT-x/internal/tmux"
)

// NOTE: All test seams (ExecuteSetupCommand, CurrentBranch on Deps;
// WalkDir, StreamCopy, SyncFile, StatFileInfo, RemoveFile,
// MaxCopyDirsFileCount, MaxCopyDirsTotalBytes on CopyDeps) are injected
// via Deps/CopyDeps fields on per-instance *Service.
// Tests are safe for t.Parallel() since no package-level variables are shared.

// ---------------------------------------------------------------------------
// Mock emitter for setup script tests
// ---------------------------------------------------------------------------

type mockEmitter struct {
	// emittedEvents captures events as (name, payload) pairs.
	emittedEvents []mockEmittedEvent
}

type mockEmittedEvent struct {
	Ctx     context.Context
	Name    string
	Payload any
}

func (m *mockEmitter) Emit(name string, payload any) {
	m.emittedEvents = append(m.emittedEvents, mockEmittedEvent{
		Ctx:     context.Background(),
		Name:    name,
		Payload: payload,
	})
}

func (m *mockEmitter) EmitWithContext(ctx context.Context, name string, payload any) {
	m.emittedEvents = append(m.emittedEvents, mockEmittedEvent{
		Ctx:     ctx,
		Name:    name,
		Payload: payload,
	})
}

func (m *mockEmitter) findPayload(eventName string) map[string]any {
	for _, e := range m.emittedEvents {
		if e.Name == eventName {
			if payload, ok := e.Payload.(map[string]any); ok {
				return payload
			}
		}
	}
	return nil
}

func (m *mockEmitter) findEvent(eventName string) *mockEmittedEvent {
	for i := range m.emittedEvents {
		if m.emittedEvents[i].Name == eventName {
			return &m.emittedEvents[i]
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Test helper: minimal Service for setup script tests
// ---------------------------------------------------------------------------

func newTestServiceForSetup(t *testing.T) (*Service, *mockEmitter) {
	t.Helper()
	emitter := &mockEmitter{}
	svc := &Service{
		deps: Deps{
			Emitter:        emitter,
			IsShuttingDown: func() bool { return false },
			RequireSessions: func() (*tmux.SessionManager, error) {
				return tmux.NewSessionManager(), nil
			},
			RequireSessionsAndRouter: func() (*tmux.SessionManager, error) {
				return tmux.NewSessionManager(), nil
			},
			GetConfigSnapshot:          func() config.Config { return config.DefaultConfig() },
			RuntimeContext:             func() context.Context { return context.Background() },
			FindAvailableSessionName:   func(name string) string { return name },
			CreateSession:              func(_, _ string, _, _, _ bool) (string, error) { return "", nil },
			ApplySessionEnvFlags:       func(_ *tmux.SessionManager, _ string, _, _, _ bool) {},
			ActivateCreatedSession:     func(_ string) (tmux.SessionSnapshot, error) { return tmux.SessionSnapshot{}, nil },
			RollbackCreatedSession:     func(_ string) error { return nil },
			StoreRootPath:              func(_, _ string) error { return nil },
			RequestSnapshot:            func(_ bool) {},
			FindSessionByWorktreePath:  func(_ string) string { return "" },
			EmitWorktreeCleanupFailure: func(_, _ string, _ error) {},
			CleanupOrphanedLocalBranch: func(_ string, _ *gitpkg.Repository, _ string) {},
			SetupWGAdd:                 func(_ int) {},
			SetupWGDone:                func() {},
			RecoverBackgroundPanic:     func(_ string, _ any) bool { return false },
			// IO operations — defaults matching NewService.
			CurrentBranch: func(repo *gitpkg.Repository) (string, error) {
				return repo.CurrentBranch()
			},
			ExecuteSetupCommand: func(ctx context.Context, shell, shellFlag, script, dir string) ([]byte, error) {
				cmd := exec.CommandContext(ctx, shell, shellFlag, script)
				cmd.Dir = dir
				return cmd.CombinedOutput()
			},
			Copy: CopyDeps{
				WalkDir:               filepath.WalkDir,
				StreamCopy:            io.Copy,
				SyncFile:              func(file *os.File) error { return file.Sync() },
				StatFileInfo:          os.Stat,
				RemoveFile:            os.Remove,
				MaxCopyDirsFileCount:  10_000,
				MaxCopyDirsTotalBytes: 500 * 1024 * 1024,
			},
		},
	}
	return svc, emitter
}

// staticFileInfo provides deterministic os.FileInfo values for testing.
type staticFileInfo struct {
	name string
	size int64
	mode fs.FileMode
}

func (f staticFileInfo) Name() string       { return f.name }
func (f staticFileInfo) Size() int64        { return f.size }
func (f staticFileInfo) Mode() fs.FileMode  { return f.mode }
func (f staticFileInfo) ModTime() time.Time { return time.Time{} }
func (f staticFileInfo) IsDir() bool        { return f.mode.IsDir() }
func (f staticFileInfo) Sys() any           { return nil }

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

// ===========================================================================
// Tests moved from app_worktree_api_test.go
// (These tests reference unexported symbols in package worktree.)
// ===========================================================================

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
	t.Parallel()

	t.Run("all scripts succeed and emit success event", func(t *testing.T) {
		t.Parallel()
		svc, emitter := newTestServiceForSetup(t)

		var ran []string
		svc.deps.ExecuteSetupCommand = func(_ context.Context, _ string, _ string, script string, _ string) ([]byte, error) {
			ran = append(ran, script)
			return []byte("ok"), nil
		}

		svc.runSetupScriptsWithParentContext(nil, t.TempDir(), "session-a", "powershell.exe", []string{"echo one", "echo two"})
		if len(ran) != 2 {
			t.Fatalf("executed scripts = %d, want 2", len(ran))
		}
		eventPayload := emitter.findPayload("worktree:setup-complete")
		if eventPayload == nil {
			t.Fatal("expected worktree:setup-complete payload")
		}
		if success, _ := eventPayload["success"].(bool); !success {
			t.Fatalf("success payload = %v, want true", eventPayload["success"])
		}
	})

	t.Run("script failure stops sequence and emits failure event", func(t *testing.T) {
		t.Parallel()
		svc, emitter := newTestServiceForSetup(t)

		var ran []string
		svc.deps.ExecuteSetupCommand = func(_ context.Context, _ string, _ string, script string, _ string) ([]byte, error) {
			ran = append(ran, script)
			if script == "bad-script" {
				return []byte("boom"), errors.New("exec failed")
			}
			return []byte("ok"), nil
		}

		svc.runSetupScriptsWithParentContext(nil, t.TempDir(), "session-b", "powershell.exe", []string{"bad-script", "never-run"})
		if len(ran) != 1 {
			t.Fatalf("executed scripts = %d, want 1", len(ran))
		}
		eventPayload := emitter.findPayload("worktree:setup-complete")
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
		t.Parallel()
		svc, emitter := newTestServiceForSetup(t)

		svc.deps.ExecuteSetupCommand = func(_ context.Context, _ string, _ string, _ string, _ string) ([]byte, error) {
			return nil, context.DeadlineExceeded
		}

		svc.runSetupScriptsWithParentContext(nil, t.TempDir(), "session-c", "powershell.exe", []string{"slow-script"})
		eventPayload := emitter.findPayload("worktree:setup-complete")
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
		t.Parallel()
		svc, emitter := newTestServiceForSetup(t)

		var ran []string
		svc.deps.ExecuteSetupCommand = func(_ context.Context, _ string, _ string, script string, _ string) ([]byte, error) {
			ran = append(ran, script)
			return []byte("ok"), nil
		}

		svc.runSetupScriptsWithParentContext(nil, t.TempDir(), "session-d", "powershell.exe", []string{"echo one", "  ", "", "echo two"})
		if len(ran) != 2 {
			t.Fatalf("executed scripts = %d, want 2 (whitespace-only should be skipped)", len(ran))
		}
		if ran[0] != "echo one" || ran[1] != "echo two" {
			t.Fatalf("ran = %v, want [echo one, echo two]", ran)
		}
		eventPayload := emitter.findPayload("worktree:setup-complete")
		if eventPayload == nil {
			t.Fatal("expected worktree:setup-complete payload")
		}
		if success, _ := eventPayload["success"].(bool); !success {
			t.Fatalf("success payload = %v, want true", eventPayload["success"])
		}
	})
}

func TestRunSetupScriptsWithParentContextFallback(t *testing.T) {
	t.Parallel()

	svc, emitter := newTestServiceForSetup(t)
	// Override RuntimeContext to return nil to test the fallback path.
	svc.deps.RuntimeContext = func() context.Context { return nil }

	ran := 0
	svc.deps.ExecuteSetupCommand = func(ctx context.Context, _ string, _ string, script string, _ string) ([]byte, error) {
		if ctx == nil {
			t.Fatal("ExecuteSetupCommand received nil context")
		}
		if strings.TrimSpace(script) == "" {
			t.Fatal("ExecuteSetupCommand received empty script")
		}
		ran++
		return []byte("ok"), nil
	}

	svc.runSetupScriptsWithParentContext(nil, t.TempDir(), "session-fallback", "powershell.exe", []string{"echo one"})

	if ran != 1 {
		t.Fatalf("executed scripts = %d, want 1", ran)
	}

	event := emitter.findEvent("worktree:setup-complete")
	if event == nil {
		t.Fatal("expected worktree:setup-complete event")
	}
	if event.Ctx == nil {
		t.Fatal("expected non-nil emit context when parent/app context are nil")
	}
	eventPayload, _ := event.Payload.(map[string]any)
	if eventPayload == nil {
		t.Fatal("expected worktree:setup-complete payload")
	}
	if success, _ := eventPayload["success"].(bool); !success {
		t.Fatalf("success payload = %v, want true", eventPayload["success"])
	}
}

func TestWaitForSetupScriptsCancellation(t *testing.T) {
	if !waitForSetupScriptsCancellation(nil, 10*time.Millisecond) {
		t.Fatal("waitForSetupScriptsCancellation(nil) = false, want true")
	}

	done := make(chan struct{})
	close(done)
	if !waitForSetupScriptsCancellation(done, 10*time.Millisecond) {
		t.Fatal("waitForSetupScriptsCancellation(closed channel) = false, want true")
	}

	blocked := make(chan struct{})
	if waitForSetupScriptsCancellation(blocked, 10*time.Millisecond) {
		t.Fatal("waitForSetupScriptsCancellation(timeout channel) = true, want false")
	}
}

func TestChooseWorktreeIdentifier(t *testing.T) {
	tests := []struct {
		name       string
		branchName string
		session    string
		want       string
		wantPrefix string
	}{
		{
			name:       "uses sanitized branch name",
			branchName: "feature/team-123",
			session:    "session-a",
			want:       "feature-team-123",
		},
		{
			name:       "falls back to session when branch sanitizes to work",
			branchName: "work",
			session:    "session-a",
			want:       "session-a",
		},
		{
			name:       "falls back to timestamp when both sanitize to work",
			branchName: "!!!",
			session:    "???",
			wantPrefix: "wt-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := chooseWorktreeIdentifier(tt.branchName, tt.session)
			if tt.want != "" && got != tt.want {
				t.Fatalf("chooseWorktreeIdentifier(%q, %q) = %q, want %q", tt.branchName, tt.session, got, tt.want)
			}
			if tt.wantPrefix != "" && !strings.HasPrefix(got, tt.wantPrefix) {
				t.Fatalf("chooseWorktreeIdentifier(%q, %q) = %q, want prefix %q", tt.branchName, tt.session, got, tt.wantPrefix)
			}
		})
	}
}

func TestCopyConfigFilesToWorktree(t *testing.T) {
	t.Parallel()
	// Shared service with default IO deps — subtests do not override deps.
	svc, _ := newTestServiceForSetup(t)

	t.Run("copies files successfully", func(t *testing.T) {
		t.Parallel()
		repoDir := t.TempDir()
		wtDir := t.TempDir()

		// Create source file.
		if err := os.WriteFile(filepath.Join(repoDir, ".env"), []byte("KEY=val"), 0o644); err != nil {
			t.Fatal(err)
		}

		failures := svc.CopyConfigFilesToWorktree(repoDir, wtDir, []string{".env"})
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

	t.Run("logs warning before overwriting existing destination file", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()

		if err := os.WriteFile(filepath.Join(repoDir, ".env"), []byte("KEY=new"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(wtDir, ".env"), []byte("KEY=old"), 0o644); err != nil {
			t.Fatal(err)
		}

		logBuf := testutil.CaptureLogBuffer(t, slog.LevelDebug)

		failures := svc.CopyConfigFilesToWorktree(repoDir, wtDir, []string{".env"})
		if len(failures) != 0 {
			t.Fatalf("unexpected failures: %v", failures)
		}
		if !strings.Contains(logBuf.String(), "overwriting existing destination file from copy_files") {
			t.Fatalf("expected overwrite warning log, got logs: %q", logBuf.String())
		}
		got, err := os.ReadFile(filepath.Join(wtDir, ".env"))
		if err != nil {
			t.Fatalf("failed to read destination file: %v", err)
		}
		if string(got) != "KEY=new" {
			t.Fatalf("destination file content = %q, want %q", string(got), "KEY=new")
		}
	})

	t.Run("reports failure when destination path already exists as directory", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()

		srcPath := filepath.Join(repoDir, "config", "app.yaml")
		if err := os.MkdirAll(filepath.Dir(srcPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(srcPath, []byte("key: value"), 0o644); err != nil {
			t.Fatal(err)
		}

		dstPath := filepath.Join(wtDir, "config", "app.yaml")
		if err := os.MkdirAll(dstPath, 0o755); err != nil {
			t.Fatal(err)
		}

		failures := svc.CopyConfigFilesToWorktree(repoDir, wtDir, []string{filepath.Join("config", "app.yaml")})
		if !reflect.DeepEqual(failures, []string{filepath.Join("config", "app.yaml")}) {
			t.Fatalf("failures = %#v, want %#v", failures, []string{filepath.Join("config", "app.yaml")})
		}
		info, statErr := os.Stat(dstPath)
		if statErr != nil {
			t.Fatalf("destination path should remain directory: %v", statErr)
		}
		if !info.IsDir() {
			t.Fatal("destination path should remain directory")
		}
	})

	t.Run("skips missing source files", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()

		failures := svc.CopyConfigFilesToWorktree(repoDir, wtDir, []string{"nonexistent.env"})
		if len(failures) != 0 {
			t.Fatalf("missing files should be silently skipped, got failures: %v", failures)
		}
	})

	t.Run("rejects absolute paths", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()

		failures := svc.CopyConfigFilesToWorktree(repoDir, wtDir, []string{`C:\Windows\System32\config.sys`})
		if !reflect.DeepEqual(failures, []string{`C:\Windows\System32\config.sys`}) {
			t.Fatalf("absolute paths should be reported as failures: %v", failures)
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

		failures := svc.CopyConfigFilesToWorktree(repoDir, wtDir, []string{"../sensitive.txt"})
		if !reflect.DeepEqual(failures, []string{"../sensitive.txt"}) {
			t.Fatalf("traversal paths should be reported as failures: %v", failures)
		}
	})

	t.Run("rejects source symlink escaping repository", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()
		outsideFile := filepath.Join(t.TempDir(), "secret.env")
		if err := os.WriteFile(outsideFile, []byte("SECRET=1"), 0o644); err != nil {
			t.Fatal(err)
		}
		linkPath := filepath.Join(repoDir, ".env")
		if err := os.Symlink(outsideFile, linkPath); err != nil {
			t.Skipf("symlink not supported in this environment: %v", err)
		}

		failures := svc.CopyConfigFilesToWorktree(repoDir, wtDir, []string{".env"})
		if !reflect.DeepEqual(failures, []string{".env"}) {
			t.Fatalf("symlink escape should be reported as failure: %v", failures)
		}
		if _, err := os.Stat(filepath.Join(wtDir, ".env")); err == nil {
			t.Fatal("destination file should not be created for escaping source symlink")
		}
	})

	t.Run("rejects destination symlink escaping worktree", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()
		outsideDir := t.TempDir()

		srcDir := filepath.Join(repoDir, "config")
		if err := os.MkdirAll(srcDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcDir, "app.yaml"), []byte("key: val"), 0o644); err != nil {
			t.Fatal(err)
		}

		linkDir := filepath.Join(wtDir, "config")
		if err := os.Symlink(outsideDir, linkDir); err != nil {
			t.Skipf("symlink not supported in this environment: %v", err)
		}

		failures := svc.CopyConfigFilesToWorktree(repoDir, wtDir, []string{filepath.Join("config", "app.yaml")})
		if !reflect.DeepEqual(failures, []string{filepath.Join("config", "app.yaml")}) {
			t.Fatalf("destination symlink escape should be reported as failure: %v", failures)
		}
		if _, err := os.Stat(filepath.Join(outsideDir, "app.yaml")); err == nil {
			t.Fatal("file should not be written outside worktree via destination symlink")
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

		failures := svc.CopyConfigFilesToWorktree(repoDir, wtDir, []string{filepath.Join("config", "app.yaml")})
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

		failures := svc.CopyConfigFilesToWorktree(repoDir, wtDir, []string{})
		if len(failures) != 0 {
			t.Fatalf("empty file list should produce no failures: %v", failures)
		}
	})

	t.Run("nil file list", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()

		failures := svc.CopyConfigFilesToWorktree(repoDir, wtDir, nil)
		if len(failures) != 0 {
			t.Fatalf("nil file list should produce no failures: %v", failures)
		}
	})

	t.Run("reports configured files when repository path resolution fails", func(t *testing.T) {
		wtDir := t.TempDir()
		want := []string{".env", "config/app.yaml"}

		failures := svc.CopyConfigFilesToWorktree("\x00", wtDir, want)
		if !reflect.DeepEqual(failures, want) {
			t.Fatalf("copy failures = %v, want %v", failures, want)
		}
	})

	t.Run("reports configured files when worktree path resolution fails", func(t *testing.T) {
		repoDir := t.TempDir()
		want := []string{".env", "config/app.yaml"}

		failures := svc.CopyConfigFilesToWorktree(repoDir, "\x00", want)
		if !reflect.DeepEqual(failures, want) {
			t.Fatalf("copy failures = %v, want %v", failures, want)
		}
	})
}

func TestCopyConfigDirsToWorktree(t *testing.T) {
	t.Parallel()
	// Shared service with default IO deps for subtests that do not override deps.
	svc, _ := newTestServiceForSetup(t)

	t.Run("copies directory successfully", func(t *testing.T) {
		t.Parallel()
		repoDir := t.TempDir()
		wtDir := t.TempDir()

		// Create source directory with files.
		srcDir := filepath.Join(repoDir, ".vscode")
		if err := os.MkdirAll(srcDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcDir, "settings.json"), []byte(`{"key":"val"}`), 0o644); err != nil {
			t.Fatal(err)
		}

		failures := svc.CopyConfigDirsToWorktree(repoDir, wtDir, []string{".vscode"})
		if len(failures) != 0 {
			t.Fatalf("unexpected failures: %v", failures)
		}

		data, err := os.ReadFile(filepath.Join(wtDir, ".vscode", "settings.json"))
		if err != nil {
			t.Fatalf("failed to read destination file: %v", err)
		}
		if string(data) != `{"key":"val"}` {
			t.Errorf("destination file content = %q, want %q", string(data), `{"key":"val"}`)
		}
	})

	t.Run("copies nested directory tree", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()

		// Create multi-level directory structure.
		nestedDir := filepath.Join(repoDir, "config", "sub", "deep")
		if err := os.MkdirAll(nestedDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repoDir, "config", "root.yaml"), []byte("root"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(nestedDir, "deep.yaml"), []byte("deep"), 0o644); err != nil {
			t.Fatal(err)
		}

		failures := svc.CopyConfigDirsToWorktree(repoDir, wtDir, []string{"config"})
		if len(failures) != 0 {
			t.Fatalf("unexpected failures: %v", failures)
		}

		// Verify root file.
		data, err := os.ReadFile(filepath.Join(wtDir, "config", "root.yaml"))
		if err != nil {
			t.Fatalf("failed to read root file: %v", err)
		}
		if string(data) != "root" {
			t.Errorf("root file content = %q, want %q", string(data), "root")
		}

		// Verify deep nested file.
		data, err = os.ReadFile(filepath.Join(wtDir, "config", "sub", "deep", "deep.yaml"))
		if err != nil {
			t.Fatalf("failed to read deep file: %v", err)
		}
		if string(data) != "deep" {
			t.Errorf("deep file content = %q, want %q", string(data), "deep")
		}
	})

	t.Run("copies repository-internal file symlink during real walk", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()

		srcDir := filepath.Join(repoDir, "config")
		if err := os.MkdirAll(srcDir, 0o755); err != nil {
			t.Fatal(err)
		}
		targetFile := filepath.Join(srcDir, "target.txt")
		if err := os.WriteFile(targetFile, []byte("symlink-target"), 0o644); err != nil {
			t.Fatal(err)
		}
		linkPath := filepath.Join(srcDir, "target-link.txt")
		if err := os.Symlink(targetFile, linkPath); err != nil {
			t.Skipf("symlink not supported in this environment: %v", err)
		}

		failures := svc.CopyConfigDirsToWorktree(repoDir, wtDir, []string{"config"})
		if len(failures) != 0 {
			t.Fatalf("unexpected failures: %v", failures)
		}
		got, readErr := os.ReadFile(filepath.Join(wtDir, "config", "target-link.txt"))
		if readErr != nil {
			t.Fatalf("failed to read copied symlink path: %v", readErr)
		}
		if string(got) != "symlink-target" {
			t.Fatalf("copied symlink content = %q, want %q", string(got), "symlink-target")
		}
	})

	t.Run("creates empty directories", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()

		emptyDir := filepath.Join(repoDir, "empty-parent", "empty-child")
		if err := os.MkdirAll(emptyDir, 0o755); err != nil {
			t.Fatal(err)
		}

		failures := svc.CopyConfigDirsToWorktree(repoDir, wtDir, []string{"empty-parent"})
		if len(failures) != 0 {
			t.Fatalf("unexpected failures: %v", failures)
		}

		// Verify empty directory exists.
		info, err := os.Stat(filepath.Join(wtDir, "empty-parent", "empty-child"))
		if err != nil {
			t.Fatalf("empty directory not created: %v", err)
		}
		if !info.IsDir() {
			t.Error("expected directory, got file")
		}
	})

	t.Run("skips missing source directory", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()

		failures := svc.CopyConfigDirsToWorktree(repoDir, wtDir, []string{"nonexistent"})
		if len(failures) != 0 {
			t.Fatalf("missing dirs should be silently skipped, got failures: %v", failures)
		}
	})

	t.Run("rejects absolute paths", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()

		failures := svc.CopyConfigDirsToWorktree(repoDir, wtDir, []string{`C:\Windows\System32`})
		if !reflect.DeepEqual(failures, []string{`C:\Windows\System32`}) {
			t.Fatalf("absolute paths should be reported as failures: %v", failures)
		}
		entries, _ := os.ReadDir(wtDir)
		if len(entries) != 0 {
			t.Errorf("expected empty wtDir, got %d entries", len(entries))
		}
	})

	t.Run("rejects current directory entry", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()

		failures := svc.CopyConfigDirsToWorktree(repoDir, wtDir, []string{"."})
		if !reflect.DeepEqual(failures, []string{"."}) {
			t.Fatalf("current directory entry should be reported as failure: %v", failures)
		}
	})

	t.Run("rejects path traversal with ..", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()

		outsideDir := filepath.Join(filepath.Dir(repoDir), "sensitive-dir")
		if err := os.MkdirAll(outsideDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(outsideDir, "secret.txt"), []byte("secret"), 0o644); err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(outsideDir)

		failures := svc.CopyConfigDirsToWorktree(repoDir, wtDir, []string{"../sensitive-dir"})
		if !reflect.DeepEqual(failures, []string{"../sensitive-dir"}) {
			t.Fatalf("traversal paths should be reported as failures: %v", failures)
		}
	})

	t.Run("rejects source symlink escaping repository", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()
		outsideDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(outsideDir, "secret.env"), []byte("SECRET=1"), 0o644); err != nil {
			t.Fatal(err)
		}
		linkPath := filepath.Join(repoDir, "linked-dir")
		if err := os.Symlink(outsideDir, linkPath); err != nil {
			t.Skipf("symlink not supported in this environment: %v", err)
		}

		failures := svc.CopyConfigDirsToWorktree(repoDir, wtDir, []string{"linked-dir"})
		if !reflect.DeepEqual(failures, []string{"linked-dir"}) {
			t.Fatalf("symlink escape should be reported as failure: %v", failures)
		}
		if _, err := os.Stat(filepath.Join(wtDir, "linked-dir")); err == nil {
			t.Fatal("destination directory should not be created for escaping source symlink")
		}
	})

	t.Run("rejects destination symlink escaping worktree", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()
		outsideDir := t.TempDir()

		srcDir := filepath.Join(repoDir, "config")
		if err := os.MkdirAll(srcDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcDir, "app.yaml"), []byte("key: val"), 0o644); err != nil {
			t.Fatal(err)
		}

		linkDir := filepath.Join(wtDir, "config")
		if err := os.Symlink(outsideDir, linkDir); err != nil {
			t.Skipf("symlink not supported in this environment: %v", err)
		}

		failures := svc.CopyConfigDirsToWorktree(repoDir, wtDir, []string{"config"})
		if !reflect.DeepEqual(failures, []string{"config"}) {
			t.Fatalf("destination symlink escape should be reported as failure: %v", failures)
		}
		if _, err := os.Stat(filepath.Join(outsideDir, "app.yaml")); err == nil {
			t.Fatal("file should not be written outside worktree via destination symlink")
		}
	})

	t.Run("marks failure when nested destination directory resolves outside worktree", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()
		outsideDir := t.TempDir()

		srcNestedDir := filepath.Join(repoDir, "config", "inner")
		if err := os.MkdirAll(srcNestedDir, 0o755); err != nil {
			t.Fatal(err)
		}

		dstParent := filepath.Join(wtDir, "config")
		if err := os.MkdirAll(dstParent, 0o755); err != nil {
			t.Fatal(err)
		}
		nestedLink := filepath.Join(dstParent, "inner")
		if err := os.Symlink(outsideDir, nestedLink); err != nil {
			t.Skipf("symlink not supported in this environment: %v", err)
		}

		failures := svc.CopyConfigDirsToWorktree(repoDir, wtDir, []string{"config"})
		if !reflect.DeepEqual(failures, []string{"config"}) {
			t.Fatalf("failures = %#v, want %#v", failures, []string{"config"})
		}
		entries, readErr := os.ReadDir(outsideDir)
		if readErr != nil {
			t.Fatalf("failed to inspect outsideDir: %v", readErr)
		}
		if len(entries) != 0 {
			t.Fatalf("outsideDir should stay untouched, got %d entries", len(entries))
		}
	})

	t.Run("copies empty files", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()

		srcDir := filepath.Join(repoDir, "empty-config")
		if err := os.MkdirAll(srcDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcDir, "empty.txt"), nil, 0o644); err != nil {
			t.Fatal(err)
		}

		failures := svc.CopyConfigDirsToWorktree(repoDir, wtDir, []string{"empty-config"})
		if len(failures) != 0 {
			t.Fatalf("unexpected failures: %v", failures)
		}
		copiedPath := filepath.Join(wtDir, "empty-config", "empty.txt")
		info, err := os.Stat(copiedPath)
		if err != nil {
			t.Fatalf("copied empty file stat failed: %v", err)
		}
		if info.Size() != 0 {
			t.Fatalf("copied empty file size = %d, want 0", info.Size())
		}
	})

	t.Run("reports partial failures across multiple directories", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()

		goodDir := filepath.Join(repoDir, "good")
		if err := os.MkdirAll(goodDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(goodDir, "ok.txt"), []byte("ok"), 0o644); err != nil {
			t.Fatal(err)
		}

		badDir := "bad\x00dir"
		failures := svc.CopyConfigDirsToWorktree(repoDir, wtDir, []string{"good", badDir})
		if !reflect.DeepEqual(failures, []string{badDir}) {
			t.Fatalf("failures = %#v, want %#v", failures, []string{badDir})
		}
		copied, err := os.ReadFile(filepath.Join(wtDir, "good", "ok.txt"))
		if err != nil {
			t.Fatalf("expected successful directory copy for good entry: %v", err)
		}
		if string(copied) != "ok" {
			t.Fatalf("copied content = %q, want %q", string(copied), "ok")
		}
	})

	t.Run("aborts walk when file count limit is exceeded", func(t *testing.T) {
		t.Parallel()
		repoDir := t.TempDir()
		wtDir := t.TempDir()
		limitedDir := filepath.Join(repoDir, "limited")
		if err := os.MkdirAll(limitedDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(limitedDir, "a.txt"), []byte("a"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(limitedDir, "b.txt"), []byte("b"), 0o644); err != nil {
			t.Fatal(err)
		}

		limitSvc, _ := newTestServiceForSetup(t)
		limitSvc.deps.Copy.MaxCopyDirsFileCount = 1
		limitSvc.deps.Copy.MaxCopyDirsTotalBytes = 1024

		failures := limitSvc.CopyConfigDirsToWorktree(repoDir, wtDir, []string{"limited"})
		if !reflect.DeepEqual(failures, []string{"limited"}) {
			t.Fatalf("failures = %#v, want %#v", failures, []string{"limited"})
		}
		if _, err := os.Stat(filepath.Join(wtDir, "limited", "a.txt")); err != nil {
			t.Fatalf("expected first file copy before limit hit: %v", err)
		}
		if _, err := os.Stat(filepath.Join(wtDir, "limited", "b.txt")); err == nil {
			t.Fatal("second file should not be copied after file count limit is reached")
		}
	})

	t.Run("aborts walk when total size limit is exceeded", func(t *testing.T) {
		t.Parallel()
		repoDir := t.TempDir()
		wtDir := t.TempDir()
		limitedDir := filepath.Join(repoDir, "size-limited")
		if err := os.MkdirAll(limitedDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(limitedDir, "a.txt"), []byte("ab"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(limitedDir, "b.txt"), []byte("cd"), 0o644); err != nil {
			t.Fatal(err)
		}

		limitSvc, _ := newTestServiceForSetup(t)
		limitSvc.deps.Copy.MaxCopyDirsFileCount = 10
		limitSvc.deps.Copy.MaxCopyDirsTotalBytes = 3

		failures := limitSvc.CopyConfigDirsToWorktree(repoDir, wtDir, []string{"size-limited"})
		if !reflect.DeepEqual(failures, []string{"size-limited"}) {
			t.Fatalf("failures = %#v, want %#v", failures, []string{"size-limited"})
		}
		if _, err := os.Stat(filepath.Join(wtDir, "size-limited", "a.txt")); err != nil {
			t.Fatalf("expected first file copy before size limit hit: %v", err)
		}
		if _, err := os.Stat(filepath.Join(wtDir, "size-limited", "b.txt")); err == nil {
			t.Fatal("second file should not be copied after size limit is reached")
		}
	})

	t.Run("aborts walk when first file already exceeds total size limit", func(t *testing.T) {
		t.Parallel()
		repoDir := t.TempDir()
		wtDir := t.TempDir()
		limitedDir := filepath.Join(repoDir, "oversized")
		if err := os.MkdirAll(limitedDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(limitedDir, "big.txt"), []byte("1234"), 0o644); err != nil {
			t.Fatal(err)
		}

		limitSvc, _ := newTestServiceForSetup(t)
		limitSvc.deps.Copy.MaxCopyDirsFileCount = 10
		limitSvc.deps.Copy.MaxCopyDirsTotalBytes = 3

		failures := limitSvc.CopyConfigDirsToWorktree(repoDir, wtDir, []string{"oversized"})
		if !reflect.DeepEqual(failures, []string{"oversized"}) {
			t.Fatalf("failures = %#v, want %#v", failures, []string{"oversized"})
		}
		if _, err := os.Stat(filepath.Join(wtDir, "oversized", "big.txt")); err == nil {
			t.Fatal("file should not be copied when first file exceeds size budget")
		}
	})

	t.Run("shares file count budget across configured directories", func(t *testing.T) {
		t.Parallel()
		repoDir := t.TempDir()
		wtDir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repoDir, "dir-a"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(repoDir, "dir-b"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repoDir, "dir-a", "a.txt"), []byte("a"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repoDir, "dir-b", "b.txt"), []byte("b"), 0o644); err != nil {
			t.Fatal(err)
		}

		limitSvc, _ := newTestServiceForSetup(t)
		limitSvc.deps.Copy.MaxCopyDirsFileCount = 1
		limitSvc.deps.Copy.MaxCopyDirsTotalBytes = 1024

		failures := limitSvc.CopyConfigDirsToWorktree(repoDir, wtDir, []string{"dir-a", "dir-b"})
		if !reflect.DeepEqual(failures, []string{"dir-b"}) {
			t.Fatalf("failures = %#v, want %#v", failures, []string{"dir-b"})
		}
		if _, err := os.Stat(filepath.Join(wtDir, "dir-a", "a.txt")); err != nil {
			t.Fatalf("expected first directory file copy: %v", err)
		}
		if _, err := os.Stat(filepath.Join(wtDir, "dir-b", "b.txt")); err == nil {
			t.Fatal("second directory file should not be copied after shared budget limit")
		}
	})

	t.Run("shares total size budget across configured directories", func(t *testing.T) {
		t.Parallel()
		repoDir := t.TempDir()
		wtDir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repoDir, "size-a"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(repoDir, "size-b"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repoDir, "size-a", "a.txt"), []byte("a"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repoDir, "size-b", "b.txt"), []byte("b"), 0o644); err != nil {
			t.Fatal(err)
		}

		limitSvc, _ := newTestServiceForSetup(t)
		limitSvc.deps.Copy.MaxCopyDirsFileCount = 10
		limitSvc.deps.Copy.MaxCopyDirsTotalBytes = 1

		failures := limitSvc.CopyConfigDirsToWorktree(repoDir, wtDir, []string{"size-a", "size-b"})
		if !reflect.DeepEqual(failures, []string{"size-b"}) {
			t.Fatalf("failures = %#v, want %#v", failures, []string{"size-b"})
		}
		if _, err := os.Stat(filepath.Join(wtDir, "size-a", "a.txt")); err != nil {
			t.Fatalf("expected first directory file copy: %v", err)
		}
		if _, err := os.Stat(filepath.Join(wtDir, "size-b", "b.txt")); err == nil {
			t.Fatal("second directory file should not be copied after shared size budget limit")
		}
	})

	t.Run("marks failure when walk callback reports error and continues remaining entries", func(t *testing.T) {
		t.Parallel()
		repoDir := t.TempDir()
		wtDir := t.TempDir()
		srcDir := filepath.Join(repoDir, "walk")
		if err := os.MkdirAll(srcDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcDir, "good.txt"), []byte("ok"), 0o644); err != nil {
			t.Fatal(err)
		}

		walkSvc, _ := newTestServiceForSetup(t)
		walkSvc.deps.Copy.WalkDir = func(root string, walkFn fs.WalkDirFunc) error {
			rootInfo, rootInfoErr := os.Stat(root)
			if rootInfoErr != nil {
				return rootInfoErr
			}
			rootEntry := fs.FileInfoToDirEntry(rootInfo)
			if callErr := walkFn(root, rootEntry, nil); callErr != nil {
				if errors.Is(callErr, filepath.SkipAll) {
					return nil
				}
				return callErr
			}
			if callErr := walkFn(filepath.Join(root, "simulated-error"), nil, errors.New("simulated walk error")); callErr != nil {
				if errors.Is(callErr, filepath.SkipAll) {
					return nil
				}
				return callErr
			}

			goodPath := filepath.Join(root, "good.txt")
			goodInfo, goodInfoErr := os.Stat(goodPath)
			if goodInfoErr != nil {
				return goodInfoErr
			}
			goodEntry := fs.FileInfoToDirEntry(goodInfo)
			if callErr := walkFn(filepath.Join(root, "good.txt"), goodEntry, nil); callErr != nil {
				if errors.Is(callErr, filepath.SkipAll) {
					return nil
				}
				return callErr
			}
			return nil
		}

		failures := walkSvc.CopyConfigDirsToWorktree(repoDir, wtDir, []string{"walk"})
		if !reflect.DeepEqual(failures, []string{"walk"}) {
			t.Fatalf("failures = %#v, want %#v", failures, []string{"walk"})
		}
		if _, err := os.Stat(filepath.Join(wtDir, "walk", "good.txt")); err != nil {
			t.Fatalf("expected remaining file to be copied despite walk error: %v", err)
		}
	})

	t.Run("skips non-directory entry", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()

		// Create a regular file where a directory is expected.
		if err := os.WriteFile(filepath.Join(repoDir, "not-a-dir"), []byte("file"), 0o644); err != nil {
			t.Fatal(err)
		}

		failures := svc.CopyConfigDirsToWorktree(repoDir, wtDir, []string{"not-a-dir"})
		if len(failures) != 0 {
			t.Fatalf("non-directory entries should be skipped, not added to failures: %v", failures)
		}
	})

	t.Run("empty dir list", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()

		failures := svc.CopyConfigDirsToWorktree(repoDir, wtDir, []string{})
		if len(failures) != 0 {
			t.Fatalf("empty dir list should produce no failures: %v", failures)
		}
	})

	t.Run("nil dir list", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()

		failures := svc.CopyConfigDirsToWorktree(repoDir, wtDir, nil)
		if len(failures) != 0 {
			t.Fatalf("nil dir list should produce no failures: %v", failures)
		}
	})

	t.Run("reports dirs when repository path resolution fails", func(t *testing.T) {
		wtDir := t.TempDir()
		want := []string{".vscode", "vendor"}

		failures := svc.CopyConfigDirsToWorktree("\x00", wtDir, want)
		if !reflect.DeepEqual(failures, want) {
			t.Fatalf("copy failures = %v, want %v", failures, want)
		}
	})

	t.Run("reports dirs when worktree path resolution fails", func(t *testing.T) {
		repoDir := t.TempDir()
		want := []string{".vscode", "vendor"}

		failures := svc.CopyConfigDirsToWorktree(repoDir, "\x00", want)
		if !reflect.DeepEqual(failures, want) {
			t.Fatalf("copy failures = %v, want %v", failures, want)
		}
	})
}

func TestHandleSymlinkInWalkCopiesRepositoryInternalFileSymlink(t *testing.T) {
	t.Parallel()
	svc, _ := newTestServiceForSetup(t)

	repoDir := t.TempDir()
	wtDir := t.TempDir()

	srcDir := filepath.Join(repoDir, "config")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	targetFile := filepath.Join(srcDir, "target.txt")
	if err := os.WriteFile(targetFile, []byte("symlink-target"), 0o644); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(srcDir, "link.txt")
	if err := os.Symlink(targetFile, linkPath); err != nil {
		t.Skipf("symlink not supported in this environment: %v", err)
	}

	hadError := false
	budget := copyWalkBudget{}
	dstPath := filepath.Join(wtDir, "config", "link.txt")
	repoBase, repoErr := resolveSymlinkEvaluatedBasePath(repoDir)
	if repoErr != nil {
		t.Fatalf("resolveSymlinkEvaluatedBasePath(repoDir) error = %v", repoErr)
	}
	wtBase, wtErr := resolveSymlinkEvaluatedBasePath(wtDir)
	if wtErr != nil {
		t.Fatalf("resolveSymlinkEvaluatedBasePath(wtDir) error = %v", wtErr)
	}
	if err := svc.handleSymlinkInWalk(linkPath, dstPath, repoBase, wtBase, "config", &hadError, &budget); err != nil {
		t.Fatalf("handleSymlinkInWalk() error = %v", err)
	}
	if hadError {
		t.Fatal("hadError = true, want false")
	}
	if budget.fileCount != 1 {
		t.Fatalf("budget.fileCount = %d, want 1", budget.fileCount)
	}
	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("failed to read copied symlink target file: %v", err)
	}
	if string(got) != "symlink-target" {
		t.Fatalf("copied symlink file content = %q, want %q", string(got), "symlink-target")
	}
}

func TestCopyFileByStreaming(t *testing.T) {
	t.Parallel()

	t.Run("copies file successfully", func(t *testing.T) {
		t.Parallel()
		svc, _ := newTestServiceForSetup(t)
		srcDir := t.TempDir()
		dstDir := t.TempDir()
		srcPath := filepath.Join(srcDir, "source.txt")
		dstPath := filepath.Join(dstDir, "dest.txt")
		if err := os.WriteFile(srcPath, []byte("hello"), 0o644); err != nil {
			t.Fatal(err)
		}

		if err := svc.copyFileByStreaming(srcPath, dstPath); err != nil {
			t.Fatalf("copyFileByStreaming() error = %v", err)
		}
		got, err := os.ReadFile(dstPath)
		if err != nil {
			t.Fatalf("read destination file error = %v", err)
		}
		if string(got) != "hello" {
			t.Fatalf("destination file content = %q, want %q", string(got), "hello")
		}
	})

	t.Run("returns not-exist error when source file is missing", func(t *testing.T) {
		t.Parallel()
		svc, _ := newTestServiceForSetup(t)
		dstPath := filepath.Join(t.TempDir(), "dest.txt")
		err := svc.copyFileByStreaming(filepath.Join(t.TempDir(), "missing.txt"), dstPath)
		if err == nil {
			t.Fatal("copyFileByStreaming() expected source not-exist error")
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("copyFileByStreaming() error = %v, want not-exist", err)
		}
	})

	t.Run("returns wrapped source open error for invalid source path", func(t *testing.T) {
		t.Parallel()
		svc, _ := newTestServiceForSetup(t)
		dstPath := filepath.Join(t.TempDir(), "dest.txt")
		err := svc.copyFileByStreaming("bad\x00source.txt", dstPath)
		if err == nil {
			t.Fatal("copyFileByStreaming() expected source open failure")
		}
		if errors.Is(err, os.ErrNotExist) {
			t.Fatalf("copyFileByStreaming() error = %v, want non-not-exist open error", err)
		}
		if !strings.Contains(err.Error(), "open source file") {
			t.Fatalf("copyFileByStreaming() error = %v, want source open context", err)
		}
	})

	t.Run("removes partial destination file when stream copy fails", func(t *testing.T) {
		t.Parallel()
		svc, _ := newTestServiceForSetup(t)
		srcDir := t.TempDir()
		dstDir := t.TempDir()
		srcPath := filepath.Join(srcDir, "source.txt")
		dstPath := filepath.Join(dstDir, "dest.txt")
		if err := os.WriteFile(srcPath, []byte("hello"), 0o644); err != nil {
			t.Fatal(err)
		}

		svc.deps.Copy.StreamCopy = func(dst io.Writer, _ io.Reader) (int64, error) {
			n, _ := dst.Write([]byte("partial"))
			return int64(n), errors.New("forced copy failure")
		}
		svc.deps.Copy.SyncFile = func(_ *os.File) error { return nil }

		err := svc.copyFileByStreaming(srcPath, dstPath)
		if err == nil {
			t.Fatal("copyFileByStreaming() expected stream copy failure")
		}
		if !strings.Contains(err.Error(), "stream copy file") {
			t.Fatalf("copyFileByStreaming() error = %v, want stream copy context", err)
		}
		if _, statErr := os.Stat(dstPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("destination file should be removed after stream failure, stat err = %v", statErr)
		}
	})

	t.Run("removes destination file when sync fails", func(t *testing.T) {
		t.Parallel()
		svc, _ := newTestServiceForSetup(t)
		srcDir := t.TempDir()
		dstDir := t.TempDir()
		srcPath := filepath.Join(srcDir, "source.txt")
		dstPath := filepath.Join(dstDir, "dest.txt")
		if err := os.WriteFile(srcPath, []byte("hello"), 0o644); err != nil {
			t.Fatal(err)
		}

		svc.deps.Copy.StreamCopy = io.Copy
		svc.deps.Copy.SyncFile = func(_ *os.File) error { return errors.New("forced sync failure") }

		err := svc.copyFileByStreaming(srcPath, dstPath)
		if err == nil {
			t.Fatal("copyFileByStreaming() expected sync failure")
		}
		if !strings.Contains(err.Error(), "sync destination file") {
			t.Fatalf("copyFileByStreaming() error = %v, want sync context", err)
		}
		if _, statErr := os.Stat(dstPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("destination file should be removed after sync failure, stat err = %v", statErr)
		}
	})

	t.Run("returns error when destination file cannot be opened", func(t *testing.T) {
		t.Parallel()
		svc, _ := newTestServiceForSetup(t)
		srcDir := t.TempDir()
		dstRoot := t.TempDir()
		srcPath := filepath.Join(srcDir, "source.txt")
		if err := os.WriteFile(srcPath, []byte("hello"), 0o644); err != nil {
			t.Fatal(err)
		}
		dstPath := filepath.Join(dstRoot, "missing-parent", "dest.txt")

		err := svc.copyFileByStreaming(srcPath, dstPath)
		if err == nil {
			t.Fatal("copyFileByStreaming() expected destination open failure")
		}
		if !strings.Contains(err.Error(), "open destination file") {
			t.Fatalf("copyFileByStreaming() error = %v, want destination open context", err)
		}
	})

	t.Run("keeps synced destination file when destination close fails", func(t *testing.T) {
		t.Parallel()
		svc, _ := newTestServiceForSetup(t)
		srcDir := t.TempDir()
		dstDir := t.TempDir()
		srcPath := filepath.Join(srcDir, "source.txt")
		dstPath := filepath.Join(dstDir, "dest.txt")
		if err := os.WriteFile(srcPath, []byte("hello"), 0o644); err != nil {
			t.Fatal(err)
		}

		svc.deps.Copy.StreamCopy = io.Copy
		svc.deps.Copy.SyncFile = func(file *os.File) error {
			return file.Close()
		}

		err := svc.copyFileByStreaming(srcPath, dstPath)
		if err == nil {
			t.Fatal("copyFileByStreaming() expected destination close failure")
		}
		if !strings.Contains(err.Error(), "close destination file") {
			t.Fatalf("copyFileByStreaming() error = %v, want destination close context", err)
		}
		got, readErr := os.ReadFile(dstPath)
		if readErr != nil {
			t.Fatalf("destination file should remain after synced close failure, read error = %v", readErr)
		}
		if string(got) != "hello" {
			t.Fatalf("destination file content = %q, want %q", string(got), "hello")
		}
	})

	t.Run("joins rollback remove failure with stream copy failure", func(t *testing.T) {
		t.Parallel()
		svc, _ := newTestServiceForSetup(t)
		srcDir := t.TempDir()
		dstDir := t.TempDir()
		srcPath := filepath.Join(srcDir, "source.txt")
		dstPath := filepath.Join(dstDir, "dest.txt")
		if err := os.WriteFile(srcPath, []byte("hello"), 0o644); err != nil {
			t.Fatal(err)
		}

		svc.deps.Copy.StreamCopy = func(dst io.Writer, _ io.Reader) (int64, error) {
			n, _ := dst.Write([]byte("partial"))
			return int64(n), errors.New("forced copy failure")
		}
		svc.deps.Copy.SyncFile = func(_ *os.File) error { return nil }
		removeSentinelErr := errors.New("forced remove failure")
		svc.deps.Copy.RemoveFile = func(path string) error {
			if path != dstPath {
				t.Fatalf("remove path = %q, want %q", path, dstPath)
			}
			return removeSentinelErr
		}

		err := svc.copyFileByStreaming(srcPath, dstPath)
		if err == nil {
			t.Fatal("copyFileByStreaming() expected joined stream/remove failure")
		}
		if !strings.Contains(err.Error(), "stream copy file") {
			t.Fatalf("copyFileByStreaming() error = %v, want stream copy context", err)
		}
		if !strings.Contains(err.Error(), "remove partial destination file") {
			t.Fatalf("copyFileByStreaming() error = %v, want remove context", err)
		}
		if !errors.Is(err, removeSentinelErr) {
			t.Fatalf("copyFileByStreaming() error = %v, want joined remove sentinel", err)
		}
	})
}

func TestCloseFileAndJoinError(t *testing.T) {
	t.Run("no-op when file is nil", func(t *testing.T) {
		var retErr error
		closeFileAndJoinError(nil, "nil file", &retErr)
		if retErr != nil {
			t.Fatalf("retErr = %v, want nil", retErr)
		}
	})

	t.Run("keeps nil error when close succeeds", func(t *testing.T) {
		file, err := os.CreateTemp(t.TempDir(), "close-ok-*.tmp")
		if err != nil {
			t.Fatal(err)
		}
		var retErr error
		closeFileAndJoinError(file, "temp file", &retErr)
		if retErr != nil {
			t.Fatalf("retErr = %v, want nil", retErr)
		}
		if err := file.Close(); err == nil {
			t.Fatal("file should already be closed after closeFileAndJoinError")
		}
	})

	t.Run("sets close error when retErr is nil", func(t *testing.T) {
		file, err := os.CreateTemp(t.TempDir(), "close-fail-*.tmp")
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		var retErr error
		closeFileAndJoinError(file, "already-closed", &retErr)
		if retErr == nil {
			t.Fatal("retErr = nil, want close error")
		}
		if !strings.Contains(retErr.Error(), "close already-closed") {
			t.Fatalf("retErr = %v, want close context", retErr)
		}
	})

	t.Run("joins close error with existing error", func(t *testing.T) {
		file, err := os.CreateTemp(t.TempDir(), "close-join-*.tmp")
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		baseErr := errors.New("base error")
		retErr := baseErr
		closeFileAndJoinError(file, "already-closed", &retErr)
		if !errors.Is(retErr, baseErr) {
			t.Fatalf("retErr should include base error, got %v", retErr)
		}
		if !strings.Contains(retErr.Error(), "close already-closed") {
			t.Fatalf("retErr = %v, want close context", retErr)
		}
	})
}

func TestReserveCopyWalkBudget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		maxFiles     int
		maxBytes     int64
		initial      copyWalkBudget
		fileSize     int64
		wantCanCopy  bool
		wantErr      error
		wantBudget   copyWalkBudget
		wantHadError bool
	}{
		{
			name:         "accepts zero-byte file on exact limits",
			maxFiles:     1,
			maxBytes:     0,
			initial:      copyWalkBudget{},
			fileSize:     0,
			wantCanCopy:  true,
			wantBudget:   copyWalkBudget{fileCount: 1, totalSize: 0},
			wantHadError: false,
		},
		{
			name:         "rejects negative file size",
			maxFiles:     10,
			maxBytes:     10,
			initial:      copyWalkBudget{},
			fileSize:     -1,
			wantCanCopy:  false,
			wantBudget:   copyWalkBudget{},
			wantHadError: true,
		},
		{
			name:         "accepts when reaching file count limit exactly",
			maxFiles:     2,
			maxBytes:     100,
			initial:      copyWalkBudget{fileCount: 1, totalSize: 10},
			fileSize:     5,
			wantCanCopy:  true,
			wantBudget:   copyWalkBudget{fileCount: 2, totalSize: 15},
			wantHadError: false,
		},
		{
			name:         "rejects when exceeding file count limit",
			maxFiles:     1,
			maxBytes:     100,
			initial:      copyWalkBudget{fileCount: 1, totalSize: 10},
			fileSize:     1,
			wantCanCopy:  false,
			wantErr:      filepath.SkipAll,
			wantBudget:   copyWalkBudget{fileCount: 1, totalSize: 10},
			wantHadError: true,
		},
		{
			name:         "accepts when reaching total size limit exactly",
			maxFiles:     10,
			maxBytes:     10,
			initial:      copyWalkBudget{fileCount: 1, totalSize: 6},
			fileSize:     4,
			wantCanCopy:  true,
			wantBudget:   copyWalkBudget{fileCount: 2, totalSize: 10},
			wantHadError: false,
		},
		{
			name:         "rejects when exceeding total size limit",
			maxFiles:     10,
			maxBytes:     10,
			initial:      copyWalkBudget{fileCount: 1, totalSize: 6},
			fileSize:     5,
			wantCanCopy:  false,
			wantErr:      filepath.SkipAll,
			wantBudget:   copyWalkBudget{fileCount: 1, totalSize: 6},
			wantHadError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			budgetSvc, _ := newTestServiceForSetup(t)
			budgetSvc.deps.Copy.MaxCopyDirsFileCount = tt.maxFiles
			budgetSvc.deps.Copy.MaxCopyDirsTotalBytes = tt.maxBytes

			budget := tt.initial
			hadError := false
			canCopy, err := budgetSvc.reserveCopyWalkBudget(&budget, tt.fileSize, "entry", "src", &hadError)
			if canCopy != tt.wantCanCopy {
				t.Fatalf("canCopy = %v, want %v", canCopy, tt.wantCanCopy)
			}
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("reserveCopyWalkBudget() error = %v, want nil", err)
				}
			} else if !errors.Is(err, tt.wantErr) {
				t.Fatalf("reserveCopyWalkBudget() error = %v, want %v", err, tt.wantErr)
			}
			if budget != tt.wantBudget {
				t.Fatalf("budget = %+v, want %+v", budget, tt.wantBudget)
			}
			if hadError != tt.wantHadError {
				t.Fatalf("hadError = %v, want %v", hadError, tt.wantHadError)
			}
		})
	}
}

func TestHandleSymlinkInWalkEdgeCases(t *testing.T) {
	t.Parallel()
	// Shared service for subtests that do not override deps.
	svc, _ := newTestServiceForSetup(t)

	t.Run("marks error for broken symlink", func(t *testing.T) {
		t.Parallel()
		repoDir := t.TempDir()
		wtDir := t.TempDir()
		linkPath := filepath.Join(repoDir, "broken.txt")
		if err := os.Symlink(filepath.Join(repoDir, "missing-target.txt"), linkPath); err != nil {
			t.Skipf("symlink not supported in this environment: %v", err)
		}
		repoBase, repoErr := resolveSymlinkEvaluatedBasePath(repoDir)
		if repoErr != nil {
			t.Fatalf("resolveSymlinkEvaluatedBasePath(repoDir) error = %v", repoErr)
		}
		wtBase, wtErr := resolveSymlinkEvaluatedBasePath(wtDir)
		if wtErr != nil {
			t.Fatalf("resolveSymlinkEvaluatedBasePath(wtDir) error = %v", wtErr)
		}

		hadError := false
		budget := copyWalkBudget{}
		err := svc.handleSymlinkInWalk(linkPath, filepath.Join(wtDir, "broken.txt"), repoBase, wtBase, "config", &hadError, &budget)
		if err != nil {
			t.Fatalf("handleSymlinkInWalk() error = %v", err)
		}
		if !hadError {
			t.Fatal("hadError = false, want true for broken symlink")
		}
	})

	t.Run("marks error when stat on resolved symlink fails with non-not-exist error", func(t *testing.T) {
		t.Parallel()
		repoDir := t.TempDir()
		wtDir := t.TempDir()
		targetFile := filepath.Join(repoDir, "target.txt")
		if err := os.WriteFile(targetFile, []byte("target"), 0o644); err != nil {
			t.Fatal(err)
		}
		linkPath := filepath.Join(repoDir, "link.txt")
		if err := os.Symlink(targetFile, linkPath); err != nil {
			t.Skipf("symlink not supported in this environment: %v", err)
		}
		repoBase, repoErr := resolveSymlinkEvaluatedBasePath(repoDir)
		if repoErr != nil {
			t.Fatalf("resolveSymlinkEvaluatedBasePath(repoDir) error = %v", repoErr)
		}
		wtBase, wtErr := resolveSymlinkEvaluatedBasePath(wtDir)
		if wtErr != nil {
			t.Fatalf("resolveSymlinkEvaluatedBasePath(wtDir) error = %v", wtErr)
		}

		statSvc, _ := newTestServiceForSetup(t)
		statSvc.deps.Copy.StatFileInfo = func(path string) (os.FileInfo, error) {
			if filepath.Clean(path) == filepath.Clean(targetFile) {
				return nil, os.ErrPermission
			}
			return os.Stat(path)
		}

		hadError := false
		budget := copyWalkBudget{}
		err := statSvc.handleSymlinkInWalk(linkPath, filepath.Join(wtDir, "link.txt"), repoBase, wtBase, "config", &hadError, &budget)
		if err != nil {
			t.Fatalf("handleSymlinkInWalk() error = %v", err)
		}
		if !hadError {
			t.Fatal("hadError = false, want true for non-not-exist stat error")
		}
		if budget.fileCount != 0 || budget.totalSize != 0 {
			t.Fatalf("budget = %+v, want zero updates for stat failure", budget)
		}
	})

	t.Run("creates directory shell for internal directory symlink", func(t *testing.T) {
		t.Parallel()
		repoDir := t.TempDir()
		wtDir := t.TempDir()
		targetDir := filepath.Join(repoDir, "real-dir")
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(targetDir, "inside.txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		linkPath := filepath.Join(repoDir, "linked-dir")
		if err := os.Symlink(targetDir, linkPath); err != nil {
			t.Skipf("symlink not supported in this environment: %v", err)
		}
		repoBase, repoErr := resolveSymlinkEvaluatedBasePath(repoDir)
		if repoErr != nil {
			t.Fatalf("resolveSymlinkEvaluatedBasePath(repoDir) error = %v", repoErr)
		}
		wtBase, wtErr := resolveSymlinkEvaluatedBasePath(wtDir)
		if wtErr != nil {
			t.Fatalf("resolveSymlinkEvaluatedBasePath(wtDir) error = %v", wtErr)
		}

		hadError := false
		budget := copyWalkBudget{}
		dstPath := filepath.Join(wtDir, "linked-dir")
		err := svc.handleSymlinkInWalk(linkPath, dstPath, repoBase, wtBase, "config", &hadError, &budget)
		if err != nil {
			t.Fatalf("handleSymlinkInWalk() error = %v", err)
		}
		if hadError {
			t.Fatal("hadError = true, want false")
		}
		info, statErr := os.Stat(dstPath)
		if statErr != nil {
			t.Fatalf("expected directory shell at destination: %v", statErr)
		}
		if !info.IsDir() {
			t.Fatal("destination should be directory shell")
		}
		if _, statErr := os.Stat(filepath.Join(dstPath, "inside.txt")); statErr == nil {
			t.Fatal("directory symlink contents should not be recursively copied")
		}
	})

	t.Run("marks error when symlinked directory shell escapes via destination symlink", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()
		outsideDir := t.TempDir()

		targetDir := filepath.Join(repoDir, "real-dir")
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			t.Fatal(err)
		}
		linkPath := filepath.Join(repoDir, "linked-dir")
		if err := os.Symlink(targetDir, linkPath); err != nil {
			t.Skipf("symlink not supported in this environment: %v", err)
		}

		dstParent := filepath.Join(wtDir, "escape-parent")
		if err := os.Symlink(outsideDir, dstParent); err != nil {
			t.Skipf("symlink not supported in this environment: %v", err)
		}

		repoBase, repoErr := resolveSymlinkEvaluatedBasePath(repoDir)
		if repoErr != nil {
			t.Fatalf("resolveSymlinkEvaluatedBasePath(repoDir) error = %v", repoErr)
		}
		wtBase, wtErr := resolveSymlinkEvaluatedBasePath(wtDir)
		if wtErr != nil {
			t.Fatalf("resolveSymlinkEvaluatedBasePath(wtDir) error = %v", wtErr)
		}

		hadError := false
		budget := copyWalkBudget{}
		err := svc.handleSymlinkInWalk(linkPath, filepath.Join(dstParent, "linked-dir"), repoBase, wtBase, "config", &hadError, &budget)
		if err != nil {
			t.Fatalf("handleSymlinkInWalk() error = %v", err)
		}
		if !hadError {
			t.Fatal("hadError = false, want true for destination symlink escape")
		}
		if budget.fileCount != 0 || budget.totalSize != 0 {
			t.Fatalf("budget = %+v, want zero updates for destination symlink escape", budget)
		}
	})

	t.Run("skips non-regular symlink target without failure", func(t *testing.T) {
		t.Parallel()
		repoDir := t.TempDir()
		wtDir := t.TempDir()
		targetFile := filepath.Join(repoDir, "target.txt")
		if err := os.WriteFile(targetFile, []byte("target"), 0o644); err != nil {
			t.Fatal(err)
		}
		linkPath := filepath.Join(repoDir, "device-like-link")
		if err := os.Symlink(targetFile, linkPath); err != nil {
			t.Skipf("symlink not supported in this environment: %v", err)
		}
		repoBase, repoErr := resolveSymlinkEvaluatedBasePath(repoDir)
		if repoErr != nil {
			t.Fatalf("resolveSymlinkEvaluatedBasePath(repoDir) error = %v", repoErr)
		}
		wtBase, wtErr := resolveSymlinkEvaluatedBasePath(wtDir)
		if wtErr != nil {
			t.Fatalf("resolveSymlinkEvaluatedBasePath(wtDir) error = %v", wtErr)
		}

		statSvc, _ := newTestServiceForSetup(t)
		statSvc.deps.Copy.StatFileInfo = func(path string) (os.FileInfo, error) {
			if path == targetFile {
				return staticFileInfo{name: filepath.Base(path), mode: os.ModeDevice}, nil
			}
			return os.Stat(path)
		}

		hadError := false
		budget := copyWalkBudget{}
		dstPath := filepath.Join(wtDir, "device-like-link")
		err := statSvc.handleSymlinkInWalk(linkPath, dstPath, repoBase, wtBase, "config", &hadError, &budget)
		if err != nil {
			t.Fatalf("handleSymlinkInWalk() error = %v", err)
		}
		if hadError {
			t.Fatal("hadError = true, want false for non-regular symlink target")
		}
		if budget.fileCount != 0 || budget.totalSize != 0 {
			t.Fatalf("budget = %+v, want zero updates for skipped non-regular target", budget)
		}
		if _, statErr := os.Stat(dstPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("destination file should not be created for non-regular symlink target, stat err = %v", statErr)
		}
	})

	t.Run("skips repository external symlink without failure", func(t *testing.T) {
		t.Parallel()
		repoDir := t.TempDir()
		wtDir := t.TempDir()
		outsideDir := t.TempDir()
		outsideFile := filepath.Join(outsideDir, "secret.txt")
		if err := os.WriteFile(outsideFile, []byte("secret"), 0o644); err != nil {
			t.Fatal(err)
		}
		linkPath := filepath.Join(repoDir, "outside.txt")
		if err := os.Symlink(outsideFile, linkPath); err != nil {
			t.Skipf("symlink not supported in this environment: %v", err)
		}
		repoBase, repoErr := resolveSymlinkEvaluatedBasePath(repoDir)
		if repoErr != nil {
			t.Fatalf("resolveSymlinkEvaluatedBasePath(repoDir) error = %v", repoErr)
		}
		wtBase, wtErr := resolveSymlinkEvaluatedBasePath(wtDir)
		if wtErr != nil {
			t.Fatalf("resolveSymlinkEvaluatedBasePath(wtDir) error = %v", wtErr)
		}

		hadError := false
		budget := copyWalkBudget{}
		dstPath := filepath.Join(wtDir, "outside.txt")
		err := svc.handleSymlinkInWalk(linkPath, dstPath, repoBase, wtBase, "config", &hadError, &budget)
		if err != nil {
			t.Fatalf("handleSymlinkInWalk() error = %v", err)
		}
		if hadError {
			t.Fatal("hadError = true, want false for intentionally skipped external symlink")
		}
		if budget.fileCount != 0 || budget.totalSize != 0 {
			t.Fatalf("budget = %+v, want zero updates for skipped external symlink", budget)
		}
		if _, statErr := os.Stat(dstPath); statErr == nil {
			t.Fatal("destination file should not be created for external symlink")
		}
	})

	t.Run("marks error when budget is nil", func(t *testing.T) {
		t.Parallel()
		repoDir := t.TempDir()
		wtDir := t.TempDir()
		targetFile := filepath.Join(repoDir, "target.txt")
		if err := os.WriteFile(targetFile, []byte("target"), 0o644); err != nil {
			t.Fatal(err)
		}
		linkPath := filepath.Join(repoDir, "link.txt")
		if err := os.Symlink(targetFile, linkPath); err != nil {
			t.Skipf("symlink not supported in this environment: %v", err)
		}
		repoBase, repoErr := resolveSymlinkEvaluatedBasePath(repoDir)
		if repoErr != nil {
			t.Fatalf("resolveSymlinkEvaluatedBasePath(repoDir) error = %v", repoErr)
		}
		wtBase, wtErr := resolveSymlinkEvaluatedBasePath(wtDir)
		if wtErr != nil {
			t.Fatalf("resolveSymlinkEvaluatedBasePath(wtDir) error = %v", wtErr)
		}

		hadError := false
		err := svc.handleSymlinkInWalk(linkPath, filepath.Join(wtDir, "link.txt"), repoBase, wtBase, "config", &hadError, nil)
		if err != nil {
			t.Fatalf("handleSymlinkInWalk() error = %v", err)
		}
		if !hadError {
			t.Fatal("hadError = false, want true when budget is nil")
		}
	})
}

func TestCopyConfigDirToWorktreeWithBudgetHandlesNilBudget(t *testing.T) {
	t.Parallel()
	svc, _ := newTestServiceForSetup(t)

	repoDir := t.TempDir()
	wtDir := t.TempDir()

	srcDir := filepath.Join(repoDir, "config")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "settings.json"), []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	repoBase, repoErr := resolveSymlinkEvaluatedBasePath(repoDir)
	if repoErr != nil {
		t.Fatalf("resolveSymlinkEvaluatedBasePath(repoDir) error = %v", repoErr)
	}
	wtBase, wtErr := resolveSymlinkEvaluatedBasePath(wtDir)
	if wtErr != nil {
		t.Fatalf("resolveSymlinkEvaluatedBasePath(wtDir) error = %v", wtErr)
	}

	failed := svc.copyConfigDirToWorktreeWithBudget(repoBase, wtBase, "config", nil)
	if failed {
		t.Fatal("copyConfigDirToWorktreeWithBudget() = true, want false")
	}
	got, err := os.ReadFile(filepath.Join(wtDir, "config", "settings.json"))
	if err != nil {
		t.Fatalf("failed to read copied file: %v", err)
	}
	if string(got) != `{"ok":true}` {
		t.Fatalf("copied file content = %q, want %q", string(got), `{"ok":true}`)
	}
}

func TestCopyFileInWalkOverwritesExistingDestinationFile(t *testing.T) {
	t.Parallel()
	svc, _ := newTestServiceForSetup(t)

	wtDir := t.TempDir()
	srcPath := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(srcPath, []byte("new-value"), 0o644); err != nil {
		t.Fatal(err)
	}
	dstPath := filepath.Join(wtDir, "config", "app.txt")
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dstPath, []byte("old-value"), 0o644); err != nil {
		t.Fatal(err)
	}

	hadError := false
	wtBase, wtErr := resolveSymlinkEvaluatedBasePath(wtDir)
	if wtErr != nil {
		t.Fatalf("resolveSymlinkEvaluatedBasePath(wtDir) error = %v", wtErr)
	}
	if err := svc.copyFileInWalk(srcPath, dstPath, wtBase, "config", &hadError); err != nil {
		t.Fatalf("copyFileInWalk() error = %v", err)
	}
	if hadError {
		t.Fatal("hadError = true, want false")
	}
	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("failed to read destination file: %v", err)
	}
	if string(got) != "new-value" {
		t.Fatalf("destination file content = %q, want %q", string(got), "new-value")
	}
}

func TestCopyFileInWalkMarksFailureWhenDestinationIsDirectory(t *testing.T) {
	t.Parallel()
	svc, _ := newTestServiceForSetup(t)

	wtDir := t.TempDir()
	srcPath := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(srcPath, []byte("value"), 0o644); err != nil {
		t.Fatal(err)
	}
	dstPath := filepath.Join(wtDir, "config", "as-dir")
	if err := os.MkdirAll(dstPath, 0o755); err != nil {
		t.Fatal(err)
	}

	hadError := false
	wtBase, wtErr := resolveSymlinkEvaluatedBasePath(wtDir)
	if wtErr != nil {
		t.Fatalf("resolveSymlinkEvaluatedBasePath(wtDir) error = %v", wtErr)
	}
	if err := svc.copyFileInWalk(srcPath, dstPath, wtBase, "config", &hadError); err != nil {
		t.Fatalf("copyFileInWalk() error = %v", err)
	}
	if !hadError {
		t.Fatal("hadError = false, want true for destination directory conflict")
	}
	info, statErr := os.Stat(dstPath)
	if statErr != nil {
		t.Fatalf("failed to stat destination directory: %v", statErr)
	}
	if !info.IsDir() {
		t.Fatal("destination path should remain a directory")
	}
}

func TestCopyFileInWalkSkipsMissingSourceWithoutFailure(t *testing.T) {
	t.Parallel()
	svc, _ := newTestServiceForSetup(t)

	wtDir := t.TempDir()
	dstPath := filepath.Join(wtDir, "config", "missing.txt")
	missingSrcPath := filepath.Join(t.TempDir(), "missing.txt")

	hadError := false
	wtBase, wtErr := resolveSymlinkEvaluatedBasePath(wtDir)
	if wtErr != nil {
		t.Fatalf("resolveSymlinkEvaluatedBasePath(wtDir) error = %v", wtErr)
	}
	if err := svc.copyFileInWalk(missingSrcPath, dstPath, wtBase, "config", &hadError); err != nil {
		t.Fatalf("copyFileInWalk() error = %v", err)
	}
	if hadError {
		t.Fatal("hadError = true, want false for missing source skip")
	}
	if _, statErr := os.Stat(dstPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("destination file should not exist after missing source skip, stat err = %v", statErr)
	}
}

func TestCopyFileInWalkMarksFailureOnNonNotExistCopyError(t *testing.T) {
	t.Parallel()
	svc, _ := newTestServiceForSetup(t)

	wtDir := t.TempDir()
	dstPath := filepath.Join(wtDir, "config", "target.txt")
	invalidSrcPath := "bad\x00source.txt"

	hadError := false
	wtBase, wtErr := resolveSymlinkEvaluatedBasePath(wtDir)
	if wtErr != nil {
		t.Fatalf("resolveSymlinkEvaluatedBasePath(wtDir) error = %v", wtErr)
	}
	if err := svc.copyFileInWalk(invalidSrcPath, dstPath, wtBase, "config", &hadError); err != nil {
		t.Fatalf("copyFileInWalk() error = %v", err)
	}
	if !hadError {
		t.Fatal("hadError = false, want true for non-not-exist copy error")
	}
}

func TestRollbackPromotedWorktreeBranch(t *testing.T) {
	repoPath := testutil.CreateTempGitRepo(t)
	runGitInDir(t, repoPath, "checkout", "--detach")
	runGitInDir(t, repoPath, "checkout", "-b", "feature/rollback-target")

	repo, err := gitpkg.Open(repoPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := rollbackPromotedWorktreeBranch(repo, "feature/rollback-target"); err != nil {
		t.Fatalf("rollbackPromotedWorktreeBranch() error = %v", err)
	}

	if current := runGitInDir(t, repoPath, "branch", "--show-current"); current != "" {
		t.Fatalf("current git branch = %q, want detached HEAD", current)
	}
	branches, err := repo.ListBranches()
	if err != nil {
		t.Fatalf("ListBranches() error = %v", err)
	}
	for _, branch := range branches {
		if branch == "feature/rollback-target" {
			t.Fatalf("rollback target branch %q should be deleted", branch)
		}
	}
}

func TestRollbackPromotedWorktreeBranchReturnsCombinedError(t *testing.T) {
	repoPath := testutil.CreateTempGitRepo(t)
	repo, err := gitpkg.Open(repoPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := os.RemoveAll(filepath.Join(repoPath, ".git")); err != nil {
		t.Fatalf("RemoveAll(.git) error = %v", err)
	}

	err = rollbackPromotedWorktreeBranch(repo, "invalid branch name")
	if err == nil {
		t.Fatal("rollbackPromotedWorktreeBranch() expected combined rollback error")
	}
	if !strings.Contains(err.Error(), "failed to restore detached HEAD during promotion rollback") {
		t.Fatalf("error = %v, want detached-HEAD rollback failure", err)
	}
	if !strings.Contains(err.Error(), "failed to delete promoted branch") {
		t.Fatalf("error = %v, want branch-delete rollback failure", err)
	}
}

// ===========================================================================
// Tests that need CurrentBranch deps access (moved from app_worktree_api_test.go)
// ===========================================================================

func TestCreateSessionWithExistingWorktreeReturnsErrorWhenCurrentBranchFails(t *testing.T) {
	t.Parallel()
	repoPath := testutil.CreateTempGitRepo(t)

	sm := tmux.NewSessionManager()
	svc := &Service{
		deps: Deps{
			Emitter:        &mockEmitter{},
			IsShuttingDown: func() bool { return false },
			RequireSessions: func() (*tmux.SessionManager, error) {
				return sm, nil
			},
			RequireSessionsAndRouter: func() (*tmux.SessionManager, error) {
				return sm, nil
			},
			GetConfigSnapshot: func() config.Config {
				cfg := config.DefaultConfig()
				cfg.Worktree.Enabled = true
				return cfg
			},
			RuntimeContext:           func() context.Context { return context.Background() },
			FindAvailableSessionName: func(name string) string { return name },
			CreateSession: func(sessionDir, sessionName string, _, _, _ bool) (string, error) {
				if _, _, err := sm.CreateSession(sessionName, "0", 120, 40); err != nil {
					return "", err
				}
				return sessionName, nil
			},
			ApplySessionEnvFlags:       func(_ *tmux.SessionManager, _ string, _, _, _ bool) {},
			ActivateCreatedSession:     func(name string) (tmux.SessionSnapshot, error) { return tmux.SessionSnapshot{Name: name}, nil },
			RollbackCreatedSession:     func(_ string) error { return nil },
			StoreRootPath:              func(_, _ string) error { return nil },
			RequestSnapshot:            func(_ bool) {},
			FindSessionByWorktreePath:  func(_ string) string { return "" },
			EmitWorktreeCleanupFailure: func(_, _ string, _ error) {},
			CleanupOrphanedLocalBranch: func(_ string, _ *gitpkg.Repository, _ string) {},
			SetupWGAdd:                 func(_ int) {},
			SetupWGDone:                func() {},
			RecoverBackgroundPanic:     func(_ string, _ any) bool { return false },
			CurrentBranch: func(*gitpkg.Repository) (string, error) {
				return "", errors.New("simulated branch detection failure")
			},
			ExecuteSetupCommand: func(ctx context.Context, shell, shellFlag, script, dir string) ([]byte, error) {
				cmd := exec.CommandContext(ctx, shell, shellFlag, script)
				cmd.Dir = dir
				return cmd.CombinedOutput()
			},
			Copy: CopyDeps{
				WalkDir:               filepath.WalkDir,
				StreamCopy:            io.Copy,
				SyncFile:              func(file *os.File) error { return file.Sync() },
				StatFileInfo:          os.Stat,
				RemoveFile:            os.Remove,
				MaxCopyDirsFileCount:  10_000,
				MaxCopyDirsTotalBytes: 500 * 1024 * 1024,
			},
		},
	}

	// When IsDetachedHead returns false (normal branch) and CurrentBranch fails,
	// the error should be surfaced — not silently treated as detached.
	_, err := svc.CreateSessionWithExistingWorktree(repoPath, "existing-wt", repoPath, SessionEnvOptions{})
	if err == nil {
		t.Fatal("expected error when CurrentBranch fails on non-detached repo")
	}
	if !strings.Contains(err.Error(), "failed to detect current branch") {
		t.Fatalf("error = %v, want 'failed to detect current branch'", err)
	}
}

func TestCreateSessionWithExistingWorktreeDetectsDetachedHead(t *testing.T) {
	t.Parallel()
	repoPath := testutil.CreateTempGitRepo(t)

	// Detach HEAD in the test repo.
	runGitInDir(t, repoPath, "checkout", "--detach")

	sm := tmux.NewSessionManager()
	svc := &Service{
		deps: Deps{
			Emitter:        &mockEmitter{},
			IsShuttingDown: func() bool { return false },
			RequireSessions: func() (*tmux.SessionManager, error) {
				return sm, nil
			},
			RequireSessionsAndRouter: func() (*tmux.SessionManager, error) {
				return sm, nil
			},
			GetConfigSnapshot: func() config.Config {
				cfg := config.DefaultConfig()
				cfg.Worktree.Enabled = true
				return cfg
			},
			RuntimeContext:           func() context.Context { return context.Background() },
			FindAvailableSessionName: func(name string) string { return name },
			CreateSession: func(sessionDir, sessionName string, _, _, _ bool) (string, error) {
				if _, _, err := sm.CreateSession(sessionName, "0", 120, 40); err != nil {
					return "", err
				}
				return sessionName, nil
			},
			ApplySessionEnvFlags:       func(_ *tmux.SessionManager, _ string, _, _, _ bool) {},
			ActivateCreatedSession:     func(name string) (tmux.SessionSnapshot, error) { return tmux.SessionSnapshot{Name: name}, nil },
			RollbackCreatedSession:     func(_ string) error { return nil },
			StoreRootPath:              func(_, _ string) error { return nil },
			RequestSnapshot:            func(_ bool) {},
			FindSessionByWorktreePath:  func(_ string) string { return "" },
			EmitWorktreeCleanupFailure: func(_, _ string, _ error) {},
			CleanupOrphanedLocalBranch: func(_ string, _ *gitpkg.Repository, _ string) {},
			SetupWGAdd:                 func(_ int) {},
			SetupWGDone:                func() {},
			RecoverBackgroundPanic:     func(_ string, _ any) bool { return false },
			CurrentBranch: func(repo *gitpkg.Repository) (string, error) {
				return repo.CurrentBranch()
			},
			ExecuteSetupCommand: func(ctx context.Context, shell, shellFlag, script, dir string) ([]byte, error) {
				cmd := exec.CommandContext(ctx, shell, shellFlag, script)
				cmd.Dir = dir
				return cmd.CombinedOutput()
			},
			Copy: CopyDeps{
				WalkDir:               filepath.WalkDir,
				StreamCopy:            io.Copy,
				SyncFile:              func(file *os.File) error { return file.Sync() },
				StatFileInfo:          os.Stat,
				RemoveFile:            os.Remove,
				MaxCopyDirsFileCount:  10_000,
				MaxCopyDirsTotalBytes: 500 * 1024 * 1024,
			},
		},
	}

	snapshot, err := svc.CreateSessionWithExistingWorktree(repoPath, "detached-wt", repoPath, SessionEnvOptions{})
	if err != nil {
		t.Fatalf("CreateSessionWithExistingWorktree() error = %v", err)
	}

	info, err := sm.GetWorktreeInfo(snapshot.Name)
	if err != nil {
		t.Fatalf("GetWorktreeInfo() error = %v", err)
	}
	if info == nil {
		t.Fatal("GetWorktreeInfo() returned nil")
	}
	if !info.IsDetached {
		t.Fatal("expected IsDetached=true for detached HEAD repo")
	}
	if info.BranchName != "" {
		t.Fatalf("expected empty BranchName for detached HEAD, got %q", info.BranchName)
	}
}

func TestCreateSessionWithExistingWorktreeReturnsErrorWhenBranchDetectionFailsWithNonEmptyBranch(t *testing.T) {
	t.Parallel()
	repoPath := testutil.CreateTempGitRepo(t)

	sm := tmux.NewSessionManager()
	svc := &Service{
		deps: Deps{
			Emitter:        &mockEmitter{},
			IsShuttingDown: func() bool { return false },
			RequireSessions: func() (*tmux.SessionManager, error) {
				return sm, nil
			},
			RequireSessionsAndRouter: func() (*tmux.SessionManager, error) {
				return sm, nil
			},
			GetConfigSnapshot: func() config.Config {
				cfg := config.DefaultConfig()
				cfg.Worktree.Enabled = true
				return cfg
			},
			RuntimeContext:           func() context.Context { return context.Background() },
			FindAvailableSessionName: func(name string) string { return name },
			CreateSession: func(sessionDir, sessionName string, _, _, _ bool) (string, error) {
				if _, _, err := sm.CreateSession(sessionName, "0", 120, 40); err != nil {
					return "", err
				}
				return sessionName, nil
			},
			ApplySessionEnvFlags:       func(_ *tmux.SessionManager, _ string, _, _, _ bool) {},
			ActivateCreatedSession:     func(name string) (tmux.SessionSnapshot, error) { return tmux.SessionSnapshot{Name: name}, nil },
			RollbackCreatedSession:     func(_ string) error { return nil },
			StoreRootPath:              func(_, _ string) error { return nil },
			RequestSnapshot:            func(_ bool) {},
			FindSessionByWorktreePath:  func(_ string) string { return "" },
			EmitWorktreeCleanupFailure: func(_, _ string, _ error) {},
			CleanupOrphanedLocalBranch: func(_ string, _ *gitpkg.Repository, _ string) {},
			SetupWGAdd:                 func(_ int) {},
			SetupWGDone:                func() {},
			RecoverBackgroundPanic:     func(_ string, _ any) bool { return false },
			// Return non-empty branch name WITH an error -> should surface the error.
			CurrentBranch: func(*gitpkg.Repository) (string, error) {
				return "ambiguous-ref", errors.New("ambiguous ref detected")
			},
			ExecuteSetupCommand: func(ctx context.Context, shell, shellFlag, script, dir string) ([]byte, error) {
				cmd := exec.CommandContext(ctx, shell, shellFlag, script)
				cmd.Dir = dir
				return cmd.CombinedOutput()
			},
			Copy: CopyDeps{
				WalkDir:               filepath.WalkDir,
				StreamCopy:            io.Copy,
				SyncFile:              func(file *os.File) error { return file.Sync() },
				StatFileInfo:          os.Stat,
				RemoveFile:            os.Remove,
				MaxCopyDirsFileCount:  10_000,
				MaxCopyDirsTotalBytes: 500 * 1024 * 1024,
			},
		},
	}

	_, err := svc.CreateSessionWithExistingWorktree(repoPath, "existing-wt", repoPath, SessionEnvOptions{})
	if err == nil {
		t.Fatal("CreateSessionWithExistingWorktree() expected error when branch detection returns non-empty branch with error")
	}
	if !strings.Contains(err.Error(), "failed to detect current branch") {
		t.Fatalf("error = %v, want 'failed to detect current branch'", err)
	}
}

// ===========================================================================
// Field count guard tests
// ===========================================================================

func TestWorktreeStructFieldCounts(t *testing.T) {
	if got := reflect.TypeFor[WorktreeSessionOptions]().NumField(); got != 7 {
		t.Fatalf("WorktreeSessionOptions field count = %d, want 7; update tests for new fields", got)
	}
	if got := reflect.TypeFor[WorktreeStatus]().NumField(); got != 5 {
		t.Fatalf("WorktreeStatus field count = %d, want 5; update tests for new fields", got)
	}
	if got := reflect.TypeFor[SessionEnvOptions]().NumField(); got != 4 {
		t.Fatalf("SessionEnvOptions field count = %d, want 4; update tests for new fields", got)
	}
	if got := reflect.TypeFor[copyWalkBudget]().NumField(); got != 2 {
		t.Fatalf("copyWalkBudget field count = %d, want 2; update tests for new fields", got)
	}
	if got := reflect.TypeFor[Deps]().NumField(); got != 22 {
		t.Fatalf("Deps field count = %d, want 22; update tests for new fields", got)
	}
	if got := reflect.TypeFor[CopyDeps]().NumField(); got != 7 {
		t.Fatalf("CopyDeps field count = %d, want 7; update tests for new fields", got)
	}
}

func TestOrphanedWorktreeFieldCount(t *testing.T) {
	if got := reflect.TypeFor[OrphanedWorktree]().NumField(); got != 4 {
		t.Fatalf("OrphanedWorktree field count = %d, want 4; update tests for new fields", got)
	}
	if got := reflect.TypeFor[createWorktreeResult]().NumField(); got != 4 {
		t.Fatalf("createWorktreeResult field count = %d, want 4; update tests for new fields", got)
	}
}

func TestRemoveEmptyWtDir(t *testing.T) {
	t.Parallel()

	t.Run("removes empty .wt directory", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		wtDir := filepath.Join(dir, "myapp.wt")
		if err := os.MkdirAll(wtDir, 0o755); err != nil {
			t.Fatal(err)
		}
		wtPath := filepath.Join(wtDir, "feature-branch")
		gitpkg.RemoveEmptyWtDir(wtPath)
		if _, err := os.Stat(wtDir); !os.IsNotExist(err) {
			t.Fatalf("expected .wt directory to be removed, stat err = %v", err)
		}
	})

	t.Run("preserves non-empty .wt directory", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		wtDir := filepath.Join(dir, "myapp.wt")
		remaining := filepath.Join(wtDir, "other-branch")
		if err := os.MkdirAll(remaining, 0o755); err != nil {
			t.Fatal(err)
		}
		wtPath := filepath.Join(wtDir, "feature-branch")
		gitpkg.RemoveEmptyWtDir(wtPath)
		if _, err := os.Stat(wtDir); err != nil {
			t.Fatalf("expected .wt directory to remain, stat err = %v", err)
		}
	})

	t.Run("ignores non-.wt parent directory", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		regular := filepath.Join(dir, "regular-dir")
		if err := os.MkdirAll(regular, 0o755); err != nil {
			t.Fatal(err)
		}
		wtPath := filepath.Join(regular, "child")
		gitpkg.RemoveEmptyWtDir(wtPath)
		if _, err := os.Stat(regular); err != nil {
			t.Fatalf("expected non-.wt directory to remain, stat err = %v", err)
		}
	})

	t.Run("ignores empty path", func(t *testing.T) {
		t.Parallel()
		gitpkg.RemoveEmptyWtDir("")
	})
}

func TestCreateWorktreeForSessionPullBestEffort(t *testing.T) {
	t.Parallel()
	repoPath := testutil.CreateTempGitRepo(t)
	repo, err := gitpkg.Open(repoPath)
	if err != nil {
		t.Fatal(err)
	}

	// Create a scenario where pull will fail (no remote).
	var progressStages []string
	onProgress := func(stage, _ string) {
		progressStages = append(progressStages, stage)
	}

	result, err := createWorktreeForSession(repo, repoPath, "test-session", WorktreeSessionOptions{
		BranchName:       "test-branch",
		PullBeforeCreate: true,
	}, onProgress)
	if err != nil {
		t.Fatalf("createWorktreeForSession() unexpected error: %v", err)
	}
	if !result.PullFailed {
		t.Fatal("expected PullFailed=true when pull fails (no remote)")
	}
	if result.WtPath == "" {
		t.Fatal("expected WtPath to be non-empty")
	}
	// Should have emitted "pulling" and "creating" progress stages.
	if len(progressStages) < 2 {
		t.Fatalf("expected at least 2 progress stages, got %d: %v", len(progressStages), progressStages)
	}
	if progressStages[0] != "pulling" {
		t.Fatalf("expected first stage 'pulling', got %q", progressStages[0])
	}
	if progressStages[1] != "creating" {
		t.Fatalf("expected second stage 'creating', got %q", progressStages[1])
	}
}

func TestCreateWorktreeForSessionNoPull(t *testing.T) {
	t.Parallel()
	repoPath := testutil.CreateTempGitRepo(t)
	repo, err := gitpkg.Open(repoPath)
	if err != nil {
		t.Fatal(err)
	}

	var progressStages []string
	onProgress := func(stage, _ string) {
		progressStages = append(progressStages, stage)
	}

	result, err := createWorktreeForSession(repo, repoPath, "test-session", WorktreeSessionOptions{
		BranchName:       "test-branch-no-pull",
		PullBeforeCreate: false,
	}, onProgress)
	if err != nil {
		t.Fatalf("createWorktreeForSession() unexpected error: %v", err)
	}
	if result.PullFailed {
		t.Fatal("expected PullFailed=false when pull is not requested")
	}
	// Should not have "pulling" stage.
	for _, stage := range progressStages {
		if stage == "pulling" {
			t.Fatal("unexpected 'pulling' stage when PullBeforeCreate=false")
		}
	}
}

func TestListOrphanedWorktrees(t *testing.T) {
	t.Parallel()

	t.Run("empty repoPath returns error", func(t *testing.T) {
		t.Parallel()
		svc, _ := newTestServiceForSetup(t)
		_, err := svc.ListOrphanedWorktrees("")
		if err == nil {
			t.Fatal("expected error for empty repoPath")
		}
	})

	t.Run("no worktrees returns nil", func(t *testing.T) {
		t.Parallel()
		repoPath := testutil.CreateTempGitRepo(t)
		sm := tmux.NewSessionManager()
		svc := &Service{
			deps: Deps{
				Emitter:        &mockEmitter{},
				IsShuttingDown: func() bool { return false },
				RequireSessions: func() (*tmux.SessionManager, error) {
					return sm, nil
				},
				RequireSessionsAndRouter:   func() (*tmux.SessionManager, error) { return sm, nil },
				GetConfigSnapshot:          func() config.Config { return config.DefaultConfig() },
				RuntimeContext:             func() context.Context { return context.Background() },
				FindAvailableSessionName:   func(name string) string { return name },
				CreateSession:              func(_, _ string, _, _, _ bool) (string, error) { return "", nil },
				ApplySessionEnvFlags:       func(_ *tmux.SessionManager, _ string, _, _, _ bool) {},
				ActivateCreatedSession:     func(_ string) (tmux.SessionSnapshot, error) { return tmux.SessionSnapshot{}, nil },
				RollbackCreatedSession:     func(_ string) error { return nil },
				StoreRootPath:              func(_, _ string) error { return nil },
				RequestSnapshot:            func(_ bool) {},
				FindSessionByWorktreePath:  func(_ string) string { return "" },
				EmitWorktreeCleanupFailure: func(_, _ string, _ error) {},
				CleanupOrphanedLocalBranch: func(_ string, _ *gitpkg.Repository, _ string) {},
				SetupWGAdd:                 func(_ int) {},
				SetupWGDone:                func() {},
				RecoverBackgroundPanic:     func(_ string, _ any) bool { return false },
			},
		}

		orphans, err := svc.ListOrphanedWorktrees(repoPath)
		if err != nil {
			t.Fatalf("ListOrphanedWorktrees() error = %v", err)
		}
		if len(orphans) != 0 {
			t.Fatalf("expected no orphans, got %d", len(orphans))
		}
	})

	t.Run("detects orphaned worktree", func(t *testing.T) {
		t.Parallel()
		repoPath := testutil.CreateTempGitRepo(t)
		repo, err := gitpkg.Open(repoPath)
		if err != nil {
			t.Fatal(err)
		}

		// Create a worktree without a corresponding session.
		wtDir := gitpkg.GenerateWorktreeDirPath(repoPath)
		if err := os.MkdirAll(wtDir, 0o755); err != nil {
			t.Fatal(err)
		}
		wtPath := filepath.Join(wtDir, "orphan-branch")
		if err := repo.CreateWorktree(wtPath, "orphan-branch", "HEAD"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = repo.RemoveWorktreeForced(wtPath) })

		sm := tmux.NewSessionManager()
		svc := &Service{
			deps: Deps{
				Emitter:        &mockEmitter{},
				IsShuttingDown: func() bool { return false },
				RequireSessions: func() (*tmux.SessionManager, error) {
					return sm, nil
				},
				RequireSessionsAndRouter:   func() (*tmux.SessionManager, error) { return sm, nil },
				GetConfigSnapshot:          func() config.Config { return config.DefaultConfig() },
				RuntimeContext:             func() context.Context { return context.Background() },
				FindAvailableSessionName:   func(name string) string { return name },
				CreateSession:              func(_, _ string, _, _, _ bool) (string, error) { return "", nil },
				ApplySessionEnvFlags:       func(_ *tmux.SessionManager, _ string, _, _, _ bool) {},
				ActivateCreatedSession:     func(_ string) (tmux.SessionSnapshot, error) { return tmux.SessionSnapshot{}, nil },
				RollbackCreatedSession:     func(_ string) error { return nil },
				StoreRootPath:              func(_, _ string) error { return nil },
				RequestSnapshot:            func(_ bool) {},
				FindSessionByWorktreePath:  func(_ string) string { return "" },
				EmitWorktreeCleanupFailure: func(_, _ string, _ error) {},
				CleanupOrphanedLocalBranch: func(_ string, _ *gitpkg.Repository, _ string) {},
				SetupWGAdd:                 func(_ int) {},
				SetupWGDone:                func() {},
				RecoverBackgroundPanic:     func(_ string, _ any) bool { return false },
			},
		}

		orphans, err := svc.ListOrphanedWorktrees(repoPath)
		if err != nil {
			t.Fatalf("ListOrphanedWorktrees() error = %v", err)
		}
		if len(orphans) != 1 {
			t.Fatalf("expected 1 orphan, got %d", len(orphans))
		}
		if orphans[0].BranchName != "orphan-branch" {
			t.Fatalf("expected branch 'orphan-branch', got %q", orphans[0].BranchName)
		}
		// Verify HasChanges is reported (clean worktree → false).
		if orphans[0].HasChanges {
			t.Fatal("expected HasChanges=false for clean orphan worktree")
		}
		// Verify Health is attached.
		if orphans[0].Health == nil {
			t.Fatal("expected Health to be non-nil for orphan worktree")
		}
		if !orphans[0].Health.IsHealthy {
			t.Fatalf("expected healthy orphan worktree, got issues: %v", orphans[0].Health.Issues)
		}
	})

	t.Run("active session worktree excluded from orphans", func(t *testing.T) {
		t.Parallel()
		repoPath := testutil.CreateTempGitRepo(t)
		repo, err := gitpkg.Open(repoPath)
		if err != nil {
			t.Fatal(err)
		}

		wtDir := gitpkg.GenerateWorktreeDirPath(repoPath)
		if err := os.MkdirAll(wtDir, 0o755); err != nil {
			t.Fatal(err)
		}
		wtPath := filepath.Join(wtDir, "active-branch")
		if err := repo.CreateWorktree(wtPath, "active-branch", "HEAD"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = repo.RemoveWorktreeForced(wtPath) })

		sm := tmux.NewSessionManager()
		// Register the worktree path as an active session.
		if _, _, err := sm.CreateSession("test-session", "0", 120, 40); err != nil {
			t.Fatal(err)
		}
		if err := sm.SetWorktreeInfo("test-session", &tmux.SessionWorktreeInfo{
			Path:     wtPath,
			RepoPath: repoPath,
		}); err != nil {
			t.Fatal(err)
		}

		svc := &Service{
			deps: Deps{
				Emitter:        &mockEmitter{},
				IsShuttingDown: func() bool { return false },
				RequireSessions: func() (*tmux.SessionManager, error) {
					return sm, nil
				},
				RequireSessionsAndRouter:   func() (*tmux.SessionManager, error) { return sm, nil },
				GetConfigSnapshot:          func() config.Config { return config.DefaultConfig() },
				RuntimeContext:             func() context.Context { return context.Background() },
				FindAvailableSessionName:   func(name string) string { return name },
				CreateSession:              func(_, _ string, _, _, _ bool) (string, error) { return "", nil },
				ApplySessionEnvFlags:       func(_ *tmux.SessionManager, _ string, _, _, _ bool) {},
				ActivateCreatedSession:     func(_ string) (tmux.SessionSnapshot, error) { return tmux.SessionSnapshot{}, nil },
				RollbackCreatedSession:     func(_ string) error { return nil },
				StoreRootPath:              func(_, _ string) error { return nil },
				RequestSnapshot:            func(_ bool) {},
				FindSessionByWorktreePath:  func(_ string) string { return "" },
				EmitWorktreeCleanupFailure: func(_, _ string, _ error) {},
				CleanupOrphanedLocalBranch: func(_ string, _ *gitpkg.Repository, _ string) {},
				SetupWGAdd:                 func(_ int) {},
				SetupWGDone:                func() {},
				RecoverBackgroundPanic:     func(_ string, _ any) bool { return false },
			},
		}

		orphans, err := svc.ListOrphanedWorktrees(repoPath)
		if err != nil {
			t.Fatalf("ListOrphanedWorktrees() error = %v", err)
		}
		if len(orphans) != 0 {
			t.Fatalf("expected 0 orphans (active session should be excluded), got %d", len(orphans))
		}
	})

	t.Run("worktree disabled returns nil", func(t *testing.T) {
		t.Parallel()
		svc, _ := newTestServiceForSetup(t)
		svc.deps.GetConfigSnapshot = func() config.Config {
			cfg := config.DefaultConfig()
			cfg.Worktree.Enabled = false
			return cfg
		}
		orphans, err := svc.ListOrphanedWorktrees("/some/path")
		if err != nil {
			t.Fatalf("expected no error when worktree disabled, got %v", err)
		}
		if orphans != nil {
			t.Fatalf("expected nil orphans when worktree disabled, got %v", orphans)
		}
	})
}

// Verify unused imports are not present.
var _ = fmt.Sprintf
