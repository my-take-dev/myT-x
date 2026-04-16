package tmux

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"myT-x/internal/ipc"
)

func TestHandleSetOption(t *testing.T) {
	sessions := NewSessionManager()
	t.Cleanup(sessions.Close)
	router := NewCommandRouter(sessions, &captureEmitter{}, RouterOptions{ShimAvailable: true})

	setResp := router.Execute(ipc.TmuxRequest{
		Command: "set-option",
		Flags:   map[string]any{"-g": true},
		Args:    []string{"focus-events", "on"},
	})
	if setResp.ExitCode != 0 {
		t.Fatalf("set-option exit = %d, want 0, stderr=%q", setResp.ExitCode, setResp.Stderr)
	}

	showResp := router.Execute(ipc.TmuxRequest{
		Command: "show-options",
		Flags:   map[string]any{"-g": true, "-v": true},
		Args:    []string{"focus-events"},
	})
	if showResp.ExitCode != 0 {
		t.Fatalf("show-options exit = %d, want 0, stderr=%q", showResp.ExitCode, showResp.Stderr)
	}
	if showResp.Stdout != "on\n" {
		t.Fatalf("show-options stdout = %q, want %q", showResp.Stdout, "on\n")
	}

	resetResp := router.Execute(ipc.TmuxRequest{
		Command: "set-option",
		Flags:   map[string]any{"-u": true, "-g": true},
		Args:    []string{"focus-events"},
	})
	if resetResp.ExitCode != 0 {
		t.Fatalf("set-option -u exit = %d, want 0, stderr=%q", resetResp.ExitCode, resetResp.Stderr)
	}

	defaultResp := router.Execute(ipc.TmuxRequest{
		Command: "show-options",
		Flags:   map[string]any{"-g": true, "-v": true},
		Args:    []string{"focus-events"},
	})
	if defaultResp.Stdout != "off\n" {
		t.Fatalf("show-options after reset = %q, want %q", defaultResp.Stdout, "off\n")
	}
}

func TestHandleShowOptions(t *testing.T) {
	sessions := NewSessionManager()
	t.Cleanup(sessions.Close)
	router := NewCommandRouter(sessions, &captureEmitter{}, RouterOptions{ShimAvailable: true})

	tests := []struct {
		name       string
		command    string
		flags      map[string]any
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{
			name:       "show alias returns default value only",
			command:    "show",
			flags:      map[string]any{"-g": true, "-v": true},
			args:       []string{"focus-events"},
			wantCode:   0,
			wantStdout: "off\n",
		},
		{
			name:       "show-options includes option name without -v",
			command:    "show-options",
			flags:      map[string]any{"-g": true},
			args:       []string{"focus-events"},
			wantCode:   0,
			wantStdout: "focus-events off\n",
		},
		{
			name:       "unknown option is quiet with -q",
			command:    "show-options",
			flags:      map[string]any{"-q": true},
			args:       []string{"does-not-exist"},
			wantCode:   0,
			wantStdout: "",
		},
		{
			name:       "unknown option returns error without -q",
			command:    "show-options",
			args:       []string{"does-not-exist"},
			wantCode:   1,
			wantStderr: "unknown option: does-not-exist\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := router.Execute(ipc.TmuxRequest{
				Command: tt.command,
				Flags:   tt.flags,
				Args:    tt.args,
			})
			if resp.ExitCode != tt.wantCode {
				t.Fatalf("ExitCode = %d, want %d, stderr=%q", resp.ExitCode, tt.wantCode, resp.Stderr)
			}
			if resp.Stdout != tt.wantStdout {
				t.Fatalf("Stdout = %q, want %q", resp.Stdout, tt.wantStdout)
			}
			if resp.Stderr != tt.wantStderr {
				t.Fatalf("Stderr = %q, want %q", resp.Stderr, tt.wantStderr)
			}
		})
	}
}

func TestHandleCompatOptionsQuietErrorsLeaveDebugBreadcrumbs(t *testing.T) {
	sessions := NewSessionManager()
	t.Cleanup(sessions.Close)
	router := NewCommandRouter(sessions, &captureEmitter{}, RouterOptions{ShimAvailable: true})

	var logBuf bytes.Buffer
	originalLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() {
		slog.SetDefault(originalLogger)
	})

	showResp := router.Execute(ipc.TmuxRequest{
		Command: "show-options",
		Flags:   map[string]any{"-q": true},
		Args:    []string{"does-not-exist"},
	})
	if showResp.ExitCode != 0 {
		t.Fatalf("show-options -q exit = %d, want 0, stderr=%q", showResp.ExitCode, showResp.Stderr)
	}

	setResp := router.Execute(ipc.TmuxRequest{
		Command: "set-option",
		Flags:   map[string]any{"-q": true},
		Args:    []string{"focus-events"},
	})
	if setResp.ExitCode != 0 {
		t.Fatalf("set-option -q exit = %d, want 0, stderr=%q", setResp.ExitCode, setResp.Stderr)
	}

	logOutput := logBuf.String()
	if strings.Count(logOutput, "quiet compatibility option error swallowed") < 2 {
		t.Fatalf("log output = %q, want quiet error breadcrumb for both commands", logOutput)
	}
	if !strings.Contains(logOutput, "command=show-options") {
		t.Fatalf("log output = %q, want show-options breadcrumb", logOutput)
	}
	if !strings.Contains(logOutput, "command=set-option") {
		t.Fatalf("log output = %q, want set-option breadcrumb", logOutput)
	}
}

func TestHandleSetOptionScopeIsolationAndOnlyIfUnset(t *testing.T) {
	sessions := NewSessionManager()
	t.Cleanup(sessions.Close)
	if _, _, err := sessions.CreateSession("alpha", "main", 120, 40); err != nil {
		t.Fatalf("CreateSession(alpha) error = %v", err)
	}
	if _, _, err := sessions.CreateSession("beta", "main", 120, 40); err != nil {
		t.Fatalf("CreateSession(beta) error = %v", err)
	}

	router := NewCommandRouter(sessions, &captureEmitter{}, RouterOptions{ShimAvailable: true})

	resp := router.Execute(ipc.TmuxRequest{
		Command: "set-option",
		Flags:   map[string]any{"-g": true},
		Args:    []string{"focus-events", "on"},
	})
	if resp.ExitCode != 0 {
		t.Fatalf("set-option -g exit = %d, want 0, stderr=%q", resp.ExitCode, resp.Stderr)
	}

	resp = router.Execute(ipc.TmuxRequest{
		Command: "set-option",
		Flags:   map[string]any{"-t": "alpha"},
		Args:    []string{"focus-events", "off"},
	})
	if resp.ExitCode != 0 {
		t.Fatalf("set-option -t alpha exit = %d, want 0, stderr=%q", resp.ExitCode, resp.Stderr)
	}

	resp = router.Execute(ipc.TmuxRequest{
		Command: "set-option",
		Flags:   map[string]any{"-o": true, "-t": "alpha"},
		Args:    []string{"focus-events", "on"},
	})
	if resp.ExitCode != 0 {
		t.Fatalf("set-option -o -t alpha exit = %d, want 0, stderr=%q", resp.ExitCode, resp.Stderr)
	}

	alphaResp := router.Execute(ipc.TmuxRequest{
		Command: "show-options",
		Flags:   map[string]any{"-t": "alpha", "-v": true},
		Args:    []string{"focus-events"},
	})
	if alphaResp.ExitCode != 0 {
		t.Fatalf("show-options -t alpha exit = %d, want 0, stderr=%q", alphaResp.ExitCode, alphaResp.Stderr)
	}
	if alphaResp.Stdout != "off\n" {
		t.Fatalf("show-options -t alpha stdout = %q, want %q", alphaResp.Stdout, "off\n")
	}

	betaResp := router.Execute(ipc.TmuxRequest{
		Command: "show-options",
		Flags:   map[string]any{"-t": "beta", "-v": true},
		Args:    []string{"focus-events"},
	})
	if betaResp.ExitCode != 0 {
		t.Fatalf("show-options -t beta exit = %d, want 0, stderr=%q", betaResp.ExitCode, betaResp.Stderr)
	}
	if betaResp.Stdout != "on\n" {
		t.Fatalf("show-options -t beta stdout = %q, want %q", betaResp.Stdout, "on\n")
	}
}

func TestHandleSetOptionRejectsUnsupportedScopedOption(t *testing.T) {
	sessions := NewSessionManager()
	t.Cleanup(sessions.Close)
	_, pane, err := sessions.CreateSession("alpha", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession(alpha) error = %v", err)
	}

	router := NewCommandRouter(sessions, &captureEmitter{}, RouterOptions{ShimAvailable: true})
	resp := router.Execute(ipc.TmuxRequest{
		Command: "set-option",
		Flags:   map[string]any{"-p": true, "-t": formatPaneID(pane.ID)},
		Args:    []string{"pane-active-border-style", "bg=default,fg=colour33"},
	})
	if resp.ExitCode != 1 {
		t.Fatalf("set-option unsupported scoped exit = %d, want 1, stderr=%q", resp.ExitCode, resp.Stderr)
	}
	if resp.Stderr == "" {
		t.Fatal("set-option unsupported scoped should report stderr")
	}
}

func TestHandleSelectLayout(t *testing.T) {
	tests := []struct {
		name  string
		flags map[string]any
		args  []string
	}{
		{
			name: "target and preset are ignored",
			flags: map[string]any{
				"-t": "demo:0",
			},
			args: []string{"main-vertical"},
		},
		{
			name: "empty request still succeeds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewCommandRouter(NewSessionManager(), &captureEmitter{}, RouterOptions{ShimAvailable: true})

			resp := router.Execute(ipc.TmuxRequest{
				Command: "select-layout",
				Flags:   tt.flags,
				Args:    tt.args,
			})

			if resp.ExitCode != 0 {
				t.Fatalf("ExitCode = %d, want 0, stderr=%q", resp.ExitCode, resp.Stderr)
			}
			if resp.Stdout != "" {
				t.Fatalf("Stdout = %q, want empty", resp.Stdout)
			}
		})
	}
}
