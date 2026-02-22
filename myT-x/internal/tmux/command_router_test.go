package tmux

import (
	"testing"

	"myT-x/internal/ipc"
)

func TestCommandRouterHandlerMapHasNoDuplicateKeys(t *testing.T) {
	// Structural integrity: verify that all handler registrations in NewCommandRouter
	// result in the expected number of handlers. If a map literal contains duplicate keys,
	// the Go compiler silently keeps only the last entry for each key.
	// This test detects accidental key collisions by checking the handler count.
	router := NewCommandRouter(NewSessionManager(), nil, RouterOptions{ShimAvailable: true})

	// Exhaustive list of commands registered in NewCommandRouter.
	// IMPORTANT (S-34): When adding a new command handler to NewCommandRouter,
	// you MUST also add it to this list. Forgetting to do so will cause this test
	// to fail with a handler count mismatch, which is the intended safeguard
	// against silent duplicate-key overwrites in the handler map literal.
	expectedCommands := []string{
		"new-session",
		"has-session",
		"split-window",
		"send-keys",
		"select-pane",
		"list-sessions",
		"kill-session",
		"list-panes",
		"display-message",
		"activate-window",
		"attach-session",
		"kill-pane",
		"rename-session",
		"resize-pane",
		"show-environment",
		"set-environment",
		"list-windows",
		"rename-window",
		"new-window",
		"kill-window",
		"select-window",
	}

	if len(router.handlers) != len(expectedCommands) {
		t.Fatalf("handler count = %d, want %d (possible duplicate key in map literal)",
			len(router.handlers), len(expectedCommands))
	}

	for _, cmd := range expectedCommands {
		if _, ok := router.handlers[cmd]; !ok {
			t.Errorf("expected handler for command %q not found", cmd)
		}
	}
}

func TestCommandRouterUnknownCommandReturnsError(t *testing.T) {
	router := NewCommandRouter(NewSessionManager(), nil, RouterOptions{})

	resp := router.Execute(ipc.TmuxRequest{Command: "nonexistent-command"})
	if resp.ExitCode != 1 {
		t.Fatalf("ExitCode = %d, want 1", resp.ExitCode)
	}
	if resp.Stderr == "" {
		t.Fatal("Stderr should contain error for unknown command")
	}
}

func TestCommandRouterExecuteNilFlags(t *testing.T) {
	// Execute should handle nil Flags and Env without panic.
	router := NewCommandRouter(NewSessionManager(), nil, RouterOptions{})

	resp := router.Execute(ipc.TmuxRequest{
		Command: "has-session",
		Flags:   nil,
		Env:     nil,
	})
	// has-session with no -t flag should return error, not panic.
	if resp.ExitCode != 1 {
		t.Fatalf("ExitCode = %d, want 1 (missing -t flag)", resp.ExitCode)
	}
}

func TestCommandRouterEmptyCommandReturnsError(t *testing.T) {
	router := NewCommandRouter(NewSessionManager(), nil, RouterOptions{})

	resp := router.Execute(ipc.TmuxRequest{Command: ""})
	if resp.ExitCode != 1 {
		t.Fatalf("ExitCode = %d, want 1", resp.ExitCode)
	}
}

func TestCommandRouterWhitespaceCommandReturnsError(t *testing.T) {
	router := NewCommandRouter(NewSessionManager(), nil, RouterOptions{})

	resp := router.Execute(ipc.TmuxRequest{Command: "   "})
	if resp.ExitCode != 1 {
		t.Fatalf("ExitCode = %d, want 1 (whitespace-only command)", resp.ExitCode)
	}
}

func TestNewCommandRouterDefaults(t *testing.T) {
	// Verify nil arguments don't panic and produce valid defaults.
	router := NewCommandRouter(nil, nil, RouterOptions{})

	if router.sessions == nil {
		t.Fatal("sessions should not be nil (auto-created)")
	}
	if router.emitter == nil {
		t.Fatal("emitter should not be nil (noopEmitter)")
	}
	if router.opts.PipeName == "" {
		t.Fatal("PipeName should have a default value")
	}
	if router.opts.HostPID <= 0 {
		t.Fatalf("HostPID = %d, want > 0", router.opts.HostPID)
	}
}
