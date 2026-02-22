package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
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
	"slices"
	"strings"
	"testing"
	"time"
)

// NOTE: This file overrides package-level function variables
// (executeRouterRequestFn, runtimeEventsEmitFn). Do not use t.Parallel() here.
// Use stubExecuteRouterRequest() for executeRouterRequestFn stubs.
// Use stubRuntimeEventsEmit() for runtimeEventsEmitFn no-op stubs.

// stubRuntimeEventsEmit replaces runtimeEventsEmitFn with a no-op for the
// duration of the test and restores the original via t.Cleanup.
// Do NOT call t.Parallel() in tests that use this helper;
// package-level variable replacement is not concurrent-safe.
func stubRuntimeEventsEmit(t *testing.T) {
	t.Helper()
	orig := runtimeEventsEmitFn
	runtimeEventsEmitFn = func(ctx context.Context, name string, data ...any) {}
	t.Cleanup(func() { runtimeEventsEmitFn = orig })
}

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
		runtimeEventsEmitFn = func(_ context.Context, name string, data ...any) {
			if name != "worktree:setup-complete" || len(data) == 0 {
				return
			}
			payload, ok := data[0].(map[string]any)
			if ok {
				eventPayload = payload
			}
		}

		app.runSetupScriptsWithParentContext(nil, t.TempDir(), "session-a", "powershell.exe", []string{"echo one", "echo two"})
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
		runtimeEventsEmitFn = func(_ context.Context, name string, data ...any) {
			if name != "worktree:setup-complete" || len(data) == 0 {
				return
			}
			payload, ok := data[0].(map[string]any)
			if ok {
				eventPayload = payload
			}
		}

		app.runSetupScriptsWithParentContext(nil, t.TempDir(), "session-b", "powershell.exe", []string{"bad-script", "never-run"})
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
		runtimeEventsEmitFn = func(_ context.Context, name string, data ...any) {
			if name != "worktree:setup-complete" || len(data) == 0 {
				return
			}
			payload, ok := data[0].(map[string]any)
			if ok {
				eventPayload = payload
			}
		}

		app.runSetupScriptsWithParentContext(nil, t.TempDir(), "session-c", "powershell.exe", []string{"slow-script"})
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
		runtimeEventsEmitFn = func(_ context.Context, name string, data ...any) {
			if name != "worktree:setup-complete" || len(data) == 0 {
				return
			}
			payload, ok := data[0].(map[string]any)
			if ok {
				eventPayload = payload
			}
		}

		app.runSetupScriptsWithParentContext(nil, t.TempDir(), "session-d", "powershell.exe", []string{"echo one", "  ", "", "echo two"})
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

func TestRunSetupScriptsWithParentContextFallback(t *testing.T) {
	origExecuteSetup := executeSetupCommandFn
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		executeSetupCommandFn = origExecuteSetup
		runtimeEventsEmitFn = origEmit
	})

	app := NewApp()
	ran := 0
	executeSetupCommandFn = func(ctx context.Context, _ string, _ string, script string, _ string) ([]byte, error) {
		if ctx == nil {
			t.Fatal("executeSetupCommandFn received nil context")
		}
		if strings.TrimSpace(script) == "" {
			t.Fatal("executeSetupCommandFn received empty script")
		}
		ran++
		return []byte("ok"), nil
	}

	var emittedCtx context.Context
	var eventPayload map[string]any
	runtimeEventsEmitFn = func(ctx context.Context, name string, data ...any) {
		if name != "worktree:setup-complete" || len(data) == 0 {
			return
		}
		emittedCtx = ctx
		payload, ok := data[0].(map[string]any)
		if ok {
			eventPayload = payload
		}
	}

	app.runSetupScriptsWithParentContext(nil, t.TempDir(), "session-fallback", "powershell.exe", []string{"echo one"})

	if ran != 1 {
		t.Fatalf("executed scripts = %d, want 1", ran)
	}
	if emittedCtx == nil {
		t.Fatal("expected non-nil emit context when parent/app context are nil")
	}
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
			want:       "featureteam-123",
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

		failures := copyConfigFilesToWorktree(repoDir, wtDir, []string{".env"})
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

		failures := copyConfigFilesToWorktree(repoDir, wtDir, []string{filepath.Join("config", "app.yaml")})
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

		failures := copyConfigFilesToWorktree(repoDir, wtDir, []string{"nonexistent.env"})
		if len(failures) != 0 {
			t.Fatalf("missing files should be silently skipped, got failures: %v", failures)
		}
	})

	t.Run("rejects absolute paths", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()

		failures := copyConfigFilesToWorktree(repoDir, wtDir, []string{`C:\Windows\System32\config.sys`})
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

		failures := copyConfigFilesToWorktree(repoDir, wtDir, []string{"../sensitive.txt"})
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

		failures := copyConfigFilesToWorktree(repoDir, wtDir, []string{".env"})
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

		failures := copyConfigFilesToWorktree(repoDir, wtDir, []string{filepath.Join("config", "app.yaml")})
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

	t.Run("reports configured files when repository path resolution fails", func(t *testing.T) {
		wtDir := t.TempDir()
		want := []string{".env", "config/app.yaml"}

		failures := copyConfigFilesToWorktree("\x00", wtDir, want)
		if !reflect.DeepEqual(failures, want) {
			t.Fatalf("copy failures = %v, want %v", failures, want)
		}
	})

	t.Run("reports configured files when worktree path resolution fails", func(t *testing.T) {
		repoDir := t.TempDir()
		want := []string{".env", "config/app.yaml"}

		failures := copyConfigFilesToWorktree(repoDir, "\x00", want)
		if !reflect.DeepEqual(failures, want) {
			t.Fatalf("copy failures = %v, want %v", failures, want)
		}
	})
}

func TestCopyConfigDirsToWorktree(t *testing.T) {
	t.Run("copies directory successfully", func(t *testing.T) {
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

		failures := copyConfigDirsToWorktree(repoDir, wtDir, []string{".vscode"})
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

		failures := copyConfigDirsToWorktree(repoDir, wtDir, []string{"config"})
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

		failures := copyConfigDirsToWorktree(repoDir, wtDir, []string{"config"})
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

		failures := copyConfigDirsToWorktree(repoDir, wtDir, []string{"empty-parent"})
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

		failures := copyConfigDirsToWorktree(repoDir, wtDir, []string{"nonexistent"})
		if len(failures) != 0 {
			t.Fatalf("missing dirs should be silently skipped, got failures: %v", failures)
		}
	})

	t.Run("rejects absolute paths", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()

		failures := copyConfigDirsToWorktree(repoDir, wtDir, []string{`C:\Windows\System32`})
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

		failures := copyConfigDirsToWorktree(repoDir, wtDir, []string{"."})
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

		failures := copyConfigDirsToWorktree(repoDir, wtDir, []string{"../sensitive-dir"})
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

		failures := copyConfigDirsToWorktree(repoDir, wtDir, []string{"linked-dir"})
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

		failures := copyConfigDirsToWorktree(repoDir, wtDir, []string{"config"})
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

		failures := copyConfigDirsToWorktree(repoDir, wtDir, []string{"config"})
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

		failures := copyConfigDirsToWorktree(repoDir, wtDir, []string{"empty-config"})
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
		failures := copyConfigDirsToWorktree(repoDir, wtDir, []string{"good", badDir})
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

		origMaxFiles := maxCopyDirsFileCount
		origMaxBytes := maxCopyDirsTotalBytes
		maxCopyDirsFileCount = 1
		maxCopyDirsTotalBytes = 1024
		t.Cleanup(func() {
			maxCopyDirsFileCount = origMaxFiles
			maxCopyDirsTotalBytes = origMaxBytes
		})

		failures := copyConfigDirsToWorktree(repoDir, wtDir, []string{"limited"})
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

		origMaxFiles := maxCopyDirsFileCount
		origMaxBytes := maxCopyDirsTotalBytes
		maxCopyDirsFileCount = 10
		maxCopyDirsTotalBytes = 3
		t.Cleanup(func() {
			maxCopyDirsFileCount = origMaxFiles
			maxCopyDirsTotalBytes = origMaxBytes
		})

		failures := copyConfigDirsToWorktree(repoDir, wtDir, []string{"size-limited"})
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
		repoDir := t.TempDir()
		wtDir := t.TempDir()
		limitedDir := filepath.Join(repoDir, "oversized")
		if err := os.MkdirAll(limitedDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(limitedDir, "big.txt"), []byte("1234"), 0o644); err != nil {
			t.Fatal(err)
		}

		origMaxFiles := maxCopyDirsFileCount
		origMaxBytes := maxCopyDirsTotalBytes
		maxCopyDirsFileCount = 10
		maxCopyDirsTotalBytes = 3
		t.Cleanup(func() {
			maxCopyDirsFileCount = origMaxFiles
			maxCopyDirsTotalBytes = origMaxBytes
		})

		failures := copyConfigDirsToWorktree(repoDir, wtDir, []string{"oversized"})
		if !reflect.DeepEqual(failures, []string{"oversized"}) {
			t.Fatalf("failures = %#v, want %#v", failures, []string{"oversized"})
		}
		if _, err := os.Stat(filepath.Join(wtDir, "oversized", "big.txt")); err == nil {
			t.Fatal("file should not be copied when first file exceeds size budget")
		}
	})

	t.Run("shares file count budget across configured directories", func(t *testing.T) {
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

		origMaxFiles := maxCopyDirsFileCount
		origMaxBytes := maxCopyDirsTotalBytes
		maxCopyDirsFileCount = 1
		maxCopyDirsTotalBytes = 1024
		t.Cleanup(func() {
			maxCopyDirsFileCount = origMaxFiles
			maxCopyDirsTotalBytes = origMaxBytes
		})

		failures := copyConfigDirsToWorktree(repoDir, wtDir, []string{"dir-a", "dir-b"})
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

		origMaxFiles := maxCopyDirsFileCount
		origMaxBytes := maxCopyDirsTotalBytes
		maxCopyDirsFileCount = 10
		maxCopyDirsTotalBytes = 1
		t.Cleanup(func() {
			maxCopyDirsFileCount = origMaxFiles
			maxCopyDirsTotalBytes = origMaxBytes
		})

		failures := copyConfigDirsToWorktree(repoDir, wtDir, []string{"size-a", "size-b"})
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
		repoDir := t.TempDir()
		wtDir := t.TempDir()
		srcDir := filepath.Join(repoDir, "walk")
		if err := os.MkdirAll(srcDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcDir, "good.txt"), []byte("ok"), 0o644); err != nil {
			t.Fatal(err)
		}

		origWalkDir := walkDirFn
		walkDirFn = func(root string, walkFn fs.WalkDirFunc) error {
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
		t.Cleanup(func() {
			walkDirFn = origWalkDir
		})

		failures := copyConfigDirsToWorktree(repoDir, wtDir, []string{"walk"})
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

		failures := copyConfigDirsToWorktree(repoDir, wtDir, []string{"not-a-dir"})
		if len(failures) != 0 {
			t.Fatalf("non-directory entries should be skipped, not added to failures: %v", failures)
		}
	})

	t.Run("empty dir list", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()

		failures := copyConfigDirsToWorktree(repoDir, wtDir, []string{})
		if len(failures) != 0 {
			t.Fatalf("empty dir list should produce no failures: %v", failures)
		}
	})

	t.Run("nil dir list", func(t *testing.T) {
		repoDir := t.TempDir()
		wtDir := t.TempDir()

		failures := copyConfigDirsToWorktree(repoDir, wtDir, nil)
		if len(failures) != 0 {
			t.Fatalf("nil dir list should produce no failures: %v", failures)
		}
	})

	t.Run("reports dirs when repository path resolution fails", func(t *testing.T) {
		wtDir := t.TempDir()
		want := []string{".vscode", "vendor"}

		failures := copyConfigDirsToWorktree("\x00", wtDir, want)
		if !reflect.DeepEqual(failures, want) {
			t.Fatalf("copy failures = %v, want %v", failures, want)
		}
	})

	t.Run("reports dirs when worktree path resolution fails", func(t *testing.T) {
		repoDir := t.TempDir()
		want := []string{".vscode", "vendor"}

		failures := copyConfigDirsToWorktree(repoDir, "\x00", want)
		if !reflect.DeepEqual(failures, want) {
			t.Fatalf("copy failures = %v, want %v", failures, want)
		}
	})
}

func TestHandleSymlinkInWalkCopiesRepositoryInternalFileSymlink(t *testing.T) {
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
	if err := handleSymlinkInWalk(linkPath, dstPath, repoBase, wtBase, "config", &hadError, &budget); err != nil {
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
	t.Run("copies file successfully", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()
		srcPath := filepath.Join(srcDir, "source.txt")
		dstPath := filepath.Join(dstDir, "dest.txt")
		if err := os.WriteFile(srcPath, []byte("hello"), 0o644); err != nil {
			t.Fatal(err)
		}

		if err := copyFileByStreaming(srcPath, dstPath); err != nil {
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
		dstPath := filepath.Join(t.TempDir(), "dest.txt")
		err := copyFileByStreaming(filepath.Join(t.TempDir(), "missing.txt"), dstPath)
		if err == nil {
			t.Fatal("copyFileByStreaming() expected source not-exist error")
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("copyFileByStreaming() error = %v, want not-exist", err)
		}
	})

	t.Run("returns wrapped source open error for invalid source path", func(t *testing.T) {
		dstPath := filepath.Join(t.TempDir(), "dest.txt")
		err := copyFileByStreaming("bad\x00source.txt", dstPath)
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
		srcDir := t.TempDir()
		dstDir := t.TempDir()
		srcPath := filepath.Join(srcDir, "source.txt")
		dstPath := filepath.Join(dstDir, "dest.txt")
		if err := os.WriteFile(srcPath, []byte("hello"), 0o644); err != nil {
			t.Fatal(err)
		}

		origCopyFn := streamCopyFn
		origSyncFn := syncFileFn
		t.Cleanup(func() {
			streamCopyFn = origCopyFn
			syncFileFn = origSyncFn
		})
		streamCopyFn = func(dst io.Writer, _ io.Reader) (int64, error) {
			n, _ := dst.Write([]byte("partial"))
			return int64(n), errors.New("forced copy failure")
		}
		syncFileFn = func(_ *os.File) error { return nil }

		err := copyFileByStreaming(srcPath, dstPath)
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
		srcDir := t.TempDir()
		dstDir := t.TempDir()
		srcPath := filepath.Join(srcDir, "source.txt")
		dstPath := filepath.Join(dstDir, "dest.txt")
		if err := os.WriteFile(srcPath, []byte("hello"), 0o644); err != nil {
			t.Fatal(err)
		}

		origCopyFn := streamCopyFn
		origSyncFn := syncFileFn
		t.Cleanup(func() {
			streamCopyFn = origCopyFn
			syncFileFn = origSyncFn
		})
		streamCopyFn = io.Copy
		syncFileFn = func(_ *os.File) error { return errors.New("forced sync failure") }

		err := copyFileByStreaming(srcPath, dstPath)
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
		srcDir := t.TempDir()
		dstRoot := t.TempDir()
		srcPath := filepath.Join(srcDir, "source.txt")
		if err := os.WriteFile(srcPath, []byte("hello"), 0o644); err != nil {
			t.Fatal(err)
		}
		dstPath := filepath.Join(dstRoot, "missing-parent", "dest.txt")

		err := copyFileByStreaming(srcPath, dstPath)
		if err == nil {
			t.Fatal("copyFileByStreaming() expected destination open failure")
		}
		if !strings.Contains(err.Error(), "open destination file") {
			t.Fatalf("copyFileByStreaming() error = %v, want destination open context", err)
		}
	})

	t.Run("keeps synced destination file when destination close fails", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()
		srcPath := filepath.Join(srcDir, "source.txt")
		dstPath := filepath.Join(dstDir, "dest.txt")
		if err := os.WriteFile(srcPath, []byte("hello"), 0o644); err != nil {
			t.Fatal(err)
		}

		origCopyFn := streamCopyFn
		origSyncFn := syncFileFn
		t.Cleanup(func() {
			streamCopyFn = origCopyFn
			syncFileFn = origSyncFn
		})
		streamCopyFn = io.Copy
		syncFileFn = func(file *os.File) error {
			return file.Close()
		}

		err := copyFileByStreaming(srcPath, dstPath)
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
		srcDir := t.TempDir()
		dstDir := t.TempDir()
		srcPath := filepath.Join(srcDir, "source.txt")
		dstPath := filepath.Join(dstDir, "dest.txt")
		if err := os.WriteFile(srcPath, []byte("hello"), 0o644); err != nil {
			t.Fatal(err)
		}

		origCopyFn := streamCopyFn
		origSyncFn := syncFileFn
		origRemoveFn := removeFileFn
		t.Cleanup(func() {
			streamCopyFn = origCopyFn
			syncFileFn = origSyncFn
			removeFileFn = origRemoveFn
		})
		streamCopyFn = func(dst io.Writer, _ io.Reader) (int64, error) {
			n, _ := dst.Write([]byte("partial"))
			return int64(n), errors.New("forced copy failure")
		}
		syncFileFn = func(_ *os.File) error { return nil }
		removeSentinelErr := errors.New("forced remove failure")
		removeFileFn = func(path string) error {
			if path != dstPath {
				t.Fatalf("remove path = %q, want %q", path, dstPath)
			}
			return removeSentinelErr
		}

		err := copyFileByStreaming(srcPath, dstPath)
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
	origMaxFiles := maxCopyDirsFileCount
	origMaxBytes := maxCopyDirsTotalBytes
	t.Cleanup(func() {
		maxCopyDirsFileCount = origMaxFiles
		maxCopyDirsTotalBytes = origMaxBytes
	})

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
			maxCopyDirsFileCount = tt.maxFiles
			maxCopyDirsTotalBytes = tt.maxBytes

			budget := tt.initial
			hadError := false
			canCopy, err := reserveCopyWalkBudget(&budget, tt.fileSize, "entry", "src", &hadError)
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
	t.Run("marks error for broken symlink", func(t *testing.T) {
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
		err := handleSymlinkInWalk(linkPath, filepath.Join(wtDir, "broken.txt"), repoBase, wtBase, "config", &hadError, &budget)
		if err != nil {
			t.Fatalf("handleSymlinkInWalk() error = %v", err)
		}
		if !hadError {
			t.Fatal("hadError = false, want true for broken symlink")
		}
	})

	t.Run("marks error when stat on resolved symlink fails with non-not-exist error", func(t *testing.T) {
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

		origStatFn := statFileInfoFn
		t.Cleanup(func() {
			statFileInfoFn = origStatFn
		})
		statFileInfoFn = func(path string) (os.FileInfo, error) {
			if filepath.Clean(path) == filepath.Clean(targetFile) {
				return nil, os.ErrPermission
			}
			return origStatFn(path)
		}

		hadError := false
		budget := copyWalkBudget{}
		err := handleSymlinkInWalk(linkPath, filepath.Join(wtDir, "link.txt"), repoBase, wtBase, "config", &hadError, &budget)
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
		err := handleSymlinkInWalk(linkPath, dstPath, repoBase, wtBase, "config", &hadError, &budget)
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
		err := handleSymlinkInWalk(linkPath, filepath.Join(dstParent, "linked-dir"), repoBase, wtBase, "config", &hadError, &budget)
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

		origStatFn := statFileInfoFn
		t.Cleanup(func() {
			statFileInfoFn = origStatFn
		})
		statFileInfoFn = func(path string) (os.FileInfo, error) {
			if path == targetFile {
				return staticFileInfo{name: filepath.Base(path), mode: os.ModeDevice}, nil
			}
			return origStatFn(path)
		}

		hadError := false
		budget := copyWalkBudget{}
		dstPath := filepath.Join(wtDir, "device-like-link")
		err := handleSymlinkInWalk(linkPath, dstPath, repoBase, wtBase, "config", &hadError, &budget)
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
		err := handleSymlinkInWalk(linkPath, dstPath, repoBase, wtBase, "config", &hadError, &budget)
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
		err := handleSymlinkInWalk(linkPath, filepath.Join(wtDir, "link.txt"), repoBase, wtBase, "config", &hadError, nil)
		if err != nil {
			t.Fatalf("handleSymlinkInWalk() error = %v", err)
		}
		if !hadError {
			t.Fatal("hadError = false, want true when budget is nil")
		}
	})
}

func TestCopyConfigDirToWorktreeWithBudgetHandlesNilBudget(t *testing.T) {
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

	failed := copyConfigDirToWorktreeWithBudget(repoBase, wtBase, "config", nil)
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
	if err := copyFileInWalk(srcPath, dstPath, wtBase, "config", &hadError); err != nil {
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
	if err := copyFileInWalk(srcPath, dstPath, wtBase, "config", &hadError); err != nil {
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
	wtDir := t.TempDir()
	dstPath := filepath.Join(wtDir, "config", "missing.txt")
	missingSrcPath := filepath.Join(t.TempDir(), "missing.txt")

	hadError := false
	wtBase, wtErr := resolveSymlinkEvaluatedBasePath(wtDir)
	if wtErr != nil {
		t.Fatalf("resolveSymlinkEvaluatedBasePath(wtDir) error = %v", wtErr)
	}
	if err := copyFileInWalk(missingSrcPath, dstPath, wtBase, "config", &hadError); err != nil {
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
	wtDir := t.TempDir()
	dstPath := filepath.Join(wtDir, "config", "target.txt")
	invalidSrcPath := "bad\x00source.txt"

	hadError := false
	wtBase, wtErr := resolveSymlinkEvaluatedBasePath(wtDir)
	if wtErr != nil {
		t.Fatalf("resolveSymlinkEvaluatedBasePath(wtDir) error = %v", wtErr)
	}
	if err := copyFileInWalk(invalidSrcPath, dstPath, wtBase, "config", &hadError); err != nil {
		t.Fatalf("copyFileInWalk() error = %v", err)
	}
	if !hadError {
		t.Fatal("hadError = false, want true for non-not-exist copy error")
	}
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

func TestFindAvailableSessionNameBoundary(t *testing.T) {
	t.Run("suffix -2 is the first candidate tried", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		if _, _, err := app.sessions.CreateSession("alpha", "0", 120, 40); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		got := app.findAvailableSessionName("alpha")
		if got != "alpha-2" {
			t.Fatalf("findAvailableSessionName(\"alpha\") = %q, want %q", got, "alpha-2")
		}
	})

	t.Run("suffix at maxSessionNameSuffix boundary", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		// Occupy "boundary" and "boundary-2" through "boundary-99".
		if _, _, err := app.sessions.CreateSession("boundary", "0", 120, 40); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		for i := 2; i <= maxSessionNameSuffix-1; i++ {
			name := fmt.Sprintf("boundary-%d", i)
			if _, _, err := app.sessions.CreateSession(name, "0", 120, 40); err != nil {
				t.Fatalf("CreateSession(%q) error = %v", name, err)
			}
		}
		// "boundary-100" should be the last numeric candidate (maxSessionNameSuffix=100).
		got := app.findAvailableSessionName("boundary")
		if got != fmt.Sprintf("boundary-%d", maxSessionNameSuffix) {
			t.Fatalf("findAvailableSessionName at boundary = %q, want %q",
				got, fmt.Sprintf("boundary-%d", maxSessionNameSuffix))
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
		for i := 2; i <= maxSessionNameSuffix; i++ {
			name := fmt.Sprintf("ts-%d", i)
			existing[name] = struct{}{}
			if _, _, err := app.sessions.CreateSession(name, "0", 120, 40); err != nil {
				t.Fatalf("CreateSession(%q) error = %v", name, err)
			}
		}
		got := app.findAvailableSessionName("ts")
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
		got := app.findAvailableSessionName("fallback")
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

	t.Run("returns error when session name is empty", func(t *testing.T) {
		repoPath := testutil.CreateTempGitRepo(t)
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
		app.setConfigSnapshot(config.DefaultConfig())

		_, err := app.CreateSessionWithWorktree(repoPath, "   ", WorktreeSessionOptions{
			BranchName: "feature/test",
		})
		if err == nil {
			t.Fatal("CreateSessionWithWorktree() expected session-name validation error")
		}
		if !strings.Contains(err.Error(), "session name is required") {
			t.Fatalf("error = %v, want session-name validation message", err)
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

	t.Run("returns error when pull-before-create fails", func(t *testing.T) {
		repoPath := testutil.CreateTempGitRepo(t)
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
		app.setConfigSnapshot(config.DefaultConfig())

		routerCalls := 0
		stubExecuteRouterRequest(t, func(_ *tmux.CommandRouter, _ ipc.TmuxRequest) ipc.TmuxResponse {
			routerCalls++
			return ipc.TmuxResponse{ExitCode: 0}
		})

		_, err := app.CreateSessionWithWorktree(repoPath, "session-a", WorktreeSessionOptions{
			BranchName:       "feature/pull-before-create",
			PullBeforeCreate: true,
		})
		if err == nil {
			t.Fatal("CreateSessionWithWorktree() expected pull failure")
		}
		if !strings.Contains(err.Error(), "failed to pull latest changes") {
			t.Fatalf("CreateSessionWithWorktree() error = %v, want pull failure message", err)
		}
		if routerCalls != 0 {
			t.Fatalf("router call count = %d, want 0 when pull fails before session creation", routerCalls)
		}
		if got := len(app.sessions.Snapshot()); got != 0 {
			t.Fatalf("session count = %d, want 0 after pull failure", got)
		}
	})
}

func TestCreateSessionWithExistingWorktreeValidation(t *testing.T) {
	t.Run("returns error when session manager is unavailable", func(t *testing.T) {
		app := NewApp()
		app.sessions = nil
		app.router = tmux.NewCommandRouter(nil, nil, tmux.RouterOptions{})
		app.setConfigSnapshot(config.DefaultConfig())

		if _, err := app.CreateSessionWithExistingWorktree(t.TempDir(), "session-a", t.TempDir(), CreateSessionOptions{}); err == nil {
			t.Fatal("CreateSessionWithExistingWorktree() expected session manager availability error")
		}
	})

	t.Run("returns error when router is unavailable", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		app.router = nil
		app.setConfigSnapshot(config.DefaultConfig())

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
		app.setConfigSnapshot(cfg)

		if _, err := app.CreateSessionWithExistingWorktree(repoPath, "session-a", repoPath, CreateSessionOptions{}); err == nil {
			t.Fatal("CreateSessionWithExistingWorktree() expected disabled feature error")
		}
	})

	t.Run("returns error when repository path is empty", func(t *testing.T) {
		repoPath := testutil.CreateTempGitRepo(t)
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
		app.setConfigSnapshot(config.DefaultConfig())

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
		app.setConfigSnapshot(config.DefaultConfig())

		_, err := app.CreateSessionWithExistingWorktree(repoPath, "session-a", "   ", CreateSessionOptions{})
		if err == nil {
			t.Fatal("CreateSessionWithExistingWorktree() expected worktree-path validation error")
		}
		if !strings.Contains(err.Error(), "worktree path is required") {
			t.Fatalf("error = %v, want worktree path validation message", err)
		}
	})

	t.Run("returns error when session name is empty", func(t *testing.T) {
		repoPath := testutil.CreateTempGitRepo(t)
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
		app.setConfigSnapshot(config.DefaultConfig())

		_, err := app.CreateSessionWithExistingWorktree(repoPath, "   ", repoPath, CreateSessionOptions{})
		if err == nil {
			t.Fatal("CreateSessionWithExistingWorktree() expected session-name validation error")
		}
		if !strings.Contains(err.Error(), "session name is required") {
			t.Fatalf("error = %v, want session-name validation message", err)
		}
	})

	t.Run("returns error when worktree path is already in use", func(t *testing.T) {
		repoPath := testutil.CreateTempGitRepo(t)
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
		app.setConfigSnapshot(config.DefaultConfig())
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
	stubExecuteRouterRequest(t, func(_ *tmux.CommandRouter, _ ipc.TmuxRequest) ipc.TmuxResponse {
		return ipc.TmuxResponse{ExitCode: 0, Stdout: "   "}
	})

	app := NewApp()

	if _, err := app.createSessionForDirectory(nil, t.TempDir(), "session-a", CreateSessionOptions{}); err == nil {
		t.Fatal("createSessionForDirectory() expected empty-name error")
	}
}

func TestCreateSessionForDirectoryRollsBackWhenTmuxReturnsEmptyName(t *testing.T) {
	app := NewApp()
	var killSessionCalled bool
	stubExecuteRouterRequest(t, func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
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

	if _, err := app.createSessionForDirectory(nil, t.TempDir(), "session-a", CreateSessionOptions{}); err == nil {
		t.Fatal("createSessionForDirectory() expected empty-name error")
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
	app.setConfigSnapshot(config.DefaultConfig())

	events := make([]string, 0, 4)
	origEmit := runtimeEventsEmitFn
	runtimeEventsEmitFn = func(_ context.Context, name string, _ ...any) {
		events = append(events, name)
	}
	t.Cleanup(func() { runtimeEventsEmitFn = origEmit })

	stubExecuteRouterRequest(t, func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
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
	app.setConfigSnapshot(config.DefaultConfig())

	capturedWorktreePath := ""
	stubExecuteRouterRequest(t, func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
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
	app.setConfigSnapshot(config.DefaultConfig())

	capturedWorktreePath := ""
	stubExecuteRouterRequest(t, func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
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
	app.setConfigSnapshot(config.DefaultConfig())

	var capturedReq ipc.TmuxRequest
	stubExecuteRouterRequest(t, func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
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

	wantEnv := agentTeamEnvVars(snapshot.Name)
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
	app.setConfigSnapshot(config.DefaultConfig())

	var requests []ipc.TmuxRequest
	stubExecuteRouterRequest(t, func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
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
	app.setConfigSnapshot(config.DefaultConfig())

	var requests []ipc.TmuxRequest
	stubExecuteRouterRequest(t, func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
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

func TestCreateSessionWithExistingWorktreeTreatsBranchDetectionErrorAsDetached(t *testing.T) {
	repoPath := testutil.CreateTempGitRepo(t)

	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
	app.setConfigSnapshot(config.DefaultConfig())

	stubExecuteRouterRequest(t, func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
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

	originalCurrentBranchFn := currentBranchFn
	t.Cleanup(func() {
		currentBranchFn = originalCurrentBranchFn
	})

	currentBranchFn = func(*gitpkg.Repository) (string, error) {
		return "", errors.New("simulated branch detection failure")
	}

	snapshot, err := app.CreateSessionWithExistingWorktree(repoPath, "existing-wt", repoPath, CreateSessionOptions{})
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

	stubExecuteRouterRequest(t, func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
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
	})

	originalCurrentBranchFn := currentBranchFn
	t.Cleanup(func() {
		currentBranchFn = originalCurrentBranchFn
	})

	// Return non-empty branch name WITH an error -> should surface the error.
	currentBranchFn = func(*gitpkg.Repository) (string, error) {
		return "ambiguous-ref", errors.New("ambiguous ref detected")
	}

	_, err := app.CreateSessionWithExistingWorktree(repoPath, "existing-wt", repoPath, CreateSessionOptions{})
	if err == nil {
		t.Fatal("CreateSessionWithExistingWorktree() expected error when branch detection returns non-empty branch with error")
	}
	if !strings.Contains(err.Error(), "failed to detect current branch") {
		t.Fatalf("error = %v, want 'failed to detect current branch'", err)
	}
}

func TestCreateSessionWithExistingWorktreeReturnsStatErrorForInvalidPath(t *testing.T) {
	repoPath := testutil.CreateTempGitRepo(t)

	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
	app.setConfigSnapshot(config.DefaultConfig())

	_, err := app.CreateSessionWithExistingWorktree(repoPath, "existing-wt", "\x00", CreateSessionOptions{})
	if err == nil {
		t.Fatal("CreateSessionWithExistingWorktree() expected stat error for invalid worktree path")
	}
	if !strings.Contains(err.Error(), "failed to stat worktree path") {
		t.Fatalf("error = %v, want stat error message", err)
	}
}

func TestWorktreeStructFieldCounts(t *testing.T) {
	if got := reflect.TypeFor[WorktreeSessionOptions]().NumField(); got != 6 {
		t.Fatalf("WorktreeSessionOptions field count = %d, want 6; update tests for new fields", got)
	}
	if got := reflect.TypeFor[WorktreeStatus]().NumField(); got != 5 {
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
	app.setConfigSnapshot(config.DefaultConfig())

	events := make([]string, 0, 4)
	origEmit := runtimeEventsEmitFn
	runtimeEventsEmitFn = func(_ context.Context, name string, _ ...any) {
		events = append(events, name)
	}
	t.Cleanup(func() { runtimeEventsEmitFn = origEmit })

	var capturedReq ipc.TmuxRequest
	stubExecuteRouterRequest(t, func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
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

	wantEnv := agentTeamEnvVars(snapshot.Name)
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
	if got := app.findSessionByWorktreePath(`C:\Projects\myapp.wt\feature`); got != "" {
		t.Fatalf("findSessionByWorktreePath() = %q, want empty when sessions is nil", got)
	}
}
