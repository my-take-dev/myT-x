package tmux

import (
	"strings"
	"testing"

	"myT-x/internal/ipc"
)

func TestHandleRunShell(t *testing.T) {
	tests := []struct {
		name         string
		flags        map[string]any
		args         []string
		wantExitCode int
		wantStdout   string
	}{
		{
			name:         "missing command argument",
			flags:        map[string]any{},
			args:         []string{},
			wantExitCode: 1,
		},
		{
			name:         "echo command",
			flags:        map[string]any{},
			args:         []string{"echo", "hello"},
			wantExitCode: 0,
			wantStdout:   "hello",
		},
		{
			name:         "failing command",
			flags:        map[string]any{},
			args:         []string{"exit /b 42"},
			wantExitCode: 42,
		},
		{
			name:         "background returns immediately",
			flags:        map[string]any{"-b": true},
			args:         []string{"echo", "bg"},
			wantExitCode: 0,
			wantStdout:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions := NewSessionManager()
			t.Cleanup(sessions.Close)
			router := NewCommandRouter(sessions, nil, RouterOptions{})

			resp := router.Execute(ipc.TmuxRequest{
				Command: "run-shell",
				Flags:   tt.flags,
				Args:    tt.args,
			})
			if resp.ExitCode != tt.wantExitCode {
				t.Fatalf("run-shell exit code = %d, want %d, stderr = %q", resp.ExitCode, tt.wantExitCode, resp.Stderr)
			}
			if tt.wantStdout != "" && !strings.Contains(resp.Stdout, tt.wantStdout) {
				t.Fatalf("run-shell stdout %q does not contain %q", resp.Stdout, tt.wantStdout)
			}
		})
	}
}

func TestHandleRunShellTmuxCommands(t *testing.T) {
	sessions := NewSessionManager()
	t.Cleanup(sessions.Close)
	router := NewCommandRouter(sessions, nil, RouterOptions{ShimAvailable: true})

	// Create a session so list-sessions can find it.
	if _, _, err := sessions.CreateSession("test", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession error: %v", err)
	}

	// -C dispatches args as tmux commands (raw string form).
	// list-sessions requires no flags and should succeed.
	resp := router.Execute(ipc.TmuxRequest{
		Command: "run-shell",
		Flags:   map[string]any{"-C": true},
		Args:    []string{"list-sessions"},
	})
	if resp.ExitCode != 0 {
		t.Fatalf("run-shell -C exit code = %d, want 0, stderr = %q", resp.ExitCode, resp.Stderr)
	}
	if !strings.Contains(resp.Stdout, "test") {
		t.Fatalf("run-shell -C stdout should contain session name 'test', got %q", resp.Stdout)
	}
}

func TestHandleIfShell(t *testing.T) {
	tests := []struct {
		name         string
		flags        map[string]any
		args         []string
		wantExitCode int
	}{
		{
			name:         "missing arguments",
			flags:        map[string]any{},
			args:         []string{"true"},
			wantExitCode: 1,
		},
		{
			name:         "format condition true",
			flags:        map[string]any{"-F": true},
			args:         []string{"1", "has-session -t nonexistent", ""},
			wantExitCode: 1, // has-session fails for nonexistent
		},
		{
			name:         "format condition false runs else",
			flags:        map[string]any{"-F": true},
			args:         []string{"0", "has-session -t x", ""},
			wantExitCode: 0, // else is empty, succeeds
		},
		{
			name:         "background returns immediately",
			flags:        map[string]any{"-b": true, "-F": true},
			args:         []string{"1", "has-session -t nonexistent"},
			wantExitCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions := NewSessionManager()
			t.Cleanup(sessions.Close)
			router := NewCommandRouter(sessions, nil, RouterOptions{})

			resp := router.Execute(ipc.TmuxRequest{
				Command: "if-shell",
				Flags:   tt.flags,
				Args:    tt.args,
			})
			if resp.ExitCode != tt.wantExitCode {
				t.Fatalf("if-shell exit code = %d, want %d, stderr = %q", resp.ExitCode, tt.wantExitCode, resp.Stderr)
			}
		})
	}
}
