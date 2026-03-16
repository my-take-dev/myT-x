package tmux

import (
	"encoding/json"
	"strings"
	"testing"

	"myT-x/internal/ipc"
)

func TestHandleMCPResolveStdio_ResolverUnavailable(t *testing.T) {
	router := NewCommandRouter(NewSessionManager(), nil, RouterOptions{})

	resp := router.Execute(ipc.TmuxRequest{
		Command: "mcp-resolve-stdio",
		Flags: map[string]any{
			"session": "s1",
			"mcp":     "gopls",
		},
	})
	if resp.ExitCode != 1 {
		t.Fatalf("ExitCode = %d, want 1", resp.ExitCode)
	}
	if !strings.Contains(resp.Stderr, "resolver is unavailable") {
		t.Fatalf("Stderr = %q, want resolver unavailable error", resp.Stderr)
	}
}

func TestHandleMCPResolveStdio_Success(t *testing.T) {
	router := NewCommandRouter(NewSessionManager(), nil, RouterOptions{
		ResolveMCPStdio: func(sessionName, mcpName string) (MCPStdioResolution, error) {
			if sessionName != "session-a" {
				t.Fatalf("sessionName = %q, want %q", sessionName, "session-a")
			}
			if mcpName != "gopls" {
				t.Fatalf("mcpName = %q, want %q", mcpName, "gopls")
			}
			return MCPStdioResolution{
				SessionName: sessionName,
				MCPID:       "lsp-gopls",
				PipePath:    `\\.\pipe\myT-x-mcp-user-session-a-lsp-gopls`,
			}, nil
		},
	})

	resp := router.Execute(ipc.TmuxRequest{
		Command: "mcp-resolve-stdio",
		Flags: map[string]any{
			"session": "session-a",
			"mcp":     "gopls",
		},
	})
	if resp.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0, stderr=%q", resp.ExitCode, resp.Stderr)
	}
	var payload ipc.MCPStdioResolvePayload
	if err := json.Unmarshal([]byte(resp.Stdout), &payload); err != nil {
		t.Fatalf("stdout is not valid json: %v (stdout=%q)", err, resp.Stdout)
	}
	if payload.SessionName != "session-a" {
		t.Fatalf("payload.session_name = %q, want %q", payload.SessionName, "session-a")
	}
	if payload.MCPID != "lsp-gopls" {
		t.Fatalf("payload.mcp_id = %q, want %q", payload.MCPID, "lsp-gopls")
	}
	if payload.PipePath == "" {
		t.Fatal("payload.pipe_path should not be empty")
	}
}

func TestHandleMCPResolveStdio_ArgsFallback(t *testing.T) {
	router := NewCommandRouter(NewSessionManager(), nil, RouterOptions{
		ResolveMCPStdio: func(sessionName, mcpName string) (MCPStdioResolution, error) {
			return MCPStdioResolution{
				SessionName: sessionName,
				MCPID:       "lsp-gopls",
				PipePath:    `\\.\pipe\myT-x-mcp-user-session-a-lsp-gopls`,
			}, nil
		},
	})

	resp := router.Execute(ipc.TmuxRequest{
		Command: "mcp-resolve-stdio",
		Args:    []string{"session-a", "gopls"},
	})
	if resp.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0, stderr=%q", resp.ExitCode, resp.Stderr)
	}
}

func TestHandleMCPResolveStdio_RejectsMixedFlagsAndArgs(t *testing.T) {
	router := NewCommandRouter(NewSessionManager(), nil, RouterOptions{
		ResolveMCPStdio: func(string, string) (MCPStdioResolution, error) {
			t.Fatal("resolver should not be called for mixed flags/args")
			return MCPStdioResolution{}, nil
		},
	})

	resp := router.Execute(ipc.TmuxRequest{
		Command: "mcp-resolve-stdio",
		Flags: map[string]any{
			"session": "session-a",
			"mcp":     "gopls",
		},
		Args: []string{"session-a", "gopls"},
	})
	if resp.ExitCode != 1 {
		t.Fatalf("ExitCode = %d, want 1", resp.ExitCode)
	}
	if !strings.Contains(resp.Stderr, "cannot be mixed") {
		t.Fatalf("Stderr = %q, want mixed flags/args error", resp.Stderr)
	}
}

func TestResolveMCPStdioRequestTarget_RejectsNonStringFlags(t *testing.T) {
	_, _, err := resolveMCPStdioRequestTarget(ipc.TmuxRequest{
		Flags: map[string]any{
			"session": true,
			"mcp":     "gopls",
		},
	})
	if err == nil {
		t.Fatal("resolveMCPStdioRequestTarget should fail for non-string session flag")
	}
	if !strings.Contains(err.Error(), "session flag must be a string, got bool") {
		t.Fatalf("error = %v, want session flag type error", err)
	}
}

func TestResolveMCPStdioRequestTarget_FlagsOnly(t *testing.T) {
	sessionName, mcpName, err := resolveMCPStdioRequestTarget(ipc.TmuxRequest{
		Flags: map[string]any{
			"session": "session-a",
			"mcp":     "gopls",
		},
	})
	if err != nil {
		t.Fatalf("resolveMCPStdioRequestTarget flags-only error = %v", err)
	}
	if sessionName != "session-a" || mcpName != "gopls" {
		t.Fatalf("resolveMCPStdioRequestTarget flags-only = (%q, %q), want (%q, %q)", sessionName, mcpName, "session-a", "gopls")
	}
}

func TestResolveMCPStdioRequestTarget_ArgsOnly(t *testing.T) {
	sessionName, mcpName, err := resolveMCPStdioRequestTarget(ipc.TmuxRequest{
		Args: []string{"session-a", "gopls"},
	})
	if err != nil {
		t.Fatalf("resolveMCPStdioRequestTarget args-only error = %v", err)
	}
	if sessionName != "session-a" || mcpName != "gopls" {
		t.Fatalf("resolveMCPStdioRequestTarget args-only = (%q, %q), want (%q, %q)", sessionName, mcpName, "session-a", "gopls")
	}
}

func TestResolveMCPStdioRequestTarget_RejectsPartialFlagSet(t *testing.T) {
	_, _, err := resolveMCPStdioRequestTarget(ipc.TmuxRequest{
		Flags: map[string]any{
			"session": "session-a",
		},
	})
	if err == nil {
		t.Fatal("resolveMCPStdioRequestTarget should fail when only one flag is provided")
	}
	if !strings.Contains(err.Error(), "must both be provided") {
		t.Fatalf("error = %v, want partial flag set error", err)
	}
}

// --- resolve-session-by-cwd handler tests ---

func TestHandleResolveSessionByCwd_ResolverUnavailable(t *testing.T) {
	router := NewCommandRouter(NewSessionManager(), nil, RouterOptions{})
	resp := router.Execute(ipc.TmuxRequest{
		Command: "resolve-session-by-cwd",
		Flags:   map[string]any{"cwd": "/some/path"},
	})
	if resp.ExitCode != 1 {
		t.Fatalf("ExitCode = %d, want 1", resp.ExitCode)
	}
	if !strings.Contains(resp.Stderr, "resolver is unavailable") {
		t.Fatalf("Stderr = %q, want resolver unavailable", resp.Stderr)
	}
}

func TestHandleResolveSessionByCwd_Success(t *testing.T) {
	router := NewCommandRouter(NewSessionManager(), nil, RouterOptions{
		ResolveSessionByCwd: func(cwd string) (string, error) {
			if cwd != "/repo/path" {
				t.Fatalf("cwd = %q, want %q", cwd, "/repo/path")
			}
			return "my-session", nil
		},
	})
	resp := router.Execute(ipc.TmuxRequest{
		Command: "resolve-session-by-cwd",
		Flags:   map[string]any{"cwd": "/repo/path"},
	})
	if resp.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0, stderr=%q", resp.ExitCode, resp.Stderr)
	}
	if strings.TrimSpace(resp.Stdout) != "my-session" {
		t.Fatalf("Stdout = %q, want %q", resp.Stdout, "my-session")
	}
}

func TestHandleResolveSessionByCwd_MissingCwd(t *testing.T) {
	router := NewCommandRouter(NewSessionManager(), nil, RouterOptions{
		ResolveSessionByCwd: func(string) (string, error) {
			t.Fatal("resolver should not be called for missing cwd")
			return "", nil
		},
	})
	resp := router.Execute(ipc.TmuxRequest{
		Command: "resolve-session-by-cwd",
		Flags:   map[string]any{},
	})
	if resp.ExitCode != 1 {
		t.Fatalf("ExitCode = %d, want 1", resp.ExitCode)
	}
	if !strings.Contains(resp.Stderr, "cwd flag is required") {
		t.Fatalf("Stderr = %q, want cwd required error", resp.Stderr)
	}
}

func TestHandleResolveSessionByCwd_EmptyCwd(t *testing.T) {
	router := NewCommandRouter(NewSessionManager(), nil, RouterOptions{
		ResolveSessionByCwd: func(string) (string, error) {
			t.Fatal("resolver should not be called for empty cwd")
			return "", nil
		},
	})
	resp := router.Execute(ipc.TmuxRequest{
		Command: "resolve-session-by-cwd",
		Flags:   map[string]any{"cwd": "  "},
	})
	if resp.ExitCode != 1 {
		t.Fatalf("ExitCode = %d, want 1", resp.ExitCode)
	}
}

func TestHandleResolveSessionByCwd_NonStringCwd(t *testing.T) {
	router := NewCommandRouter(NewSessionManager(), nil, RouterOptions{
		ResolveSessionByCwd: func(string) (string, error) {
			t.Fatal("resolver should not be called for non-string cwd")
			return "", nil
		},
	})
	resp := router.Execute(ipc.TmuxRequest{
		Command: "resolve-session-by-cwd",
		Flags:   map[string]any{"cwd": 42},
	})
	if resp.ExitCode != 1 {
		t.Fatalf("ExitCode = %d, want 1", resp.ExitCode)
	}
	if !strings.Contains(resp.Stderr, "must be a string") {
		t.Fatalf("Stderr = %q, want type error", resp.Stderr)
	}
}

func TestResolveMCPStdioRequestTarget_RejectsWrongArgCount(t *testing.T) {
	_, _, err := resolveMCPStdioRequestTarget(ipc.TmuxRequest{
		Args: []string{"session-a"},
	})
	if err == nil {
		t.Fatal("resolveMCPStdioRequestTarget should fail when arg count is not 2")
	}
	if !strings.Contains(err.Error(), "expected 2 positional arguments, got 1") {
		t.Fatalf("error = %v, want arg count detail", err)
	}
}
