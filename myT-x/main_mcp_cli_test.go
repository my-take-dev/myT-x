package main

import (
	"bytes"
	"errors"
	"io"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"myT-x/internal/ipc"
)

// newTestMCPCLIDeps creates a mcpCLIDeps with sensible defaults for testing.
// Fields can be overridden after creation.
func newTestMCPCLIDeps() mcpCLIDeps {
	return mcpCLIDeps{
		sendIPCRequest:      func(string, ipc.TmuxRequest) (ipc.TmuxResponse, error) { return ipc.TmuxResponse{}, nil },
		platformSupported:   func() bool { return true },
		pipeName:            func() string { return `\\.\pipe\myT-x-test` },
		resolveSessionByEnv: func() string { return "" },
		resolveSessionByCwd: func(_, _ string) (string, error) { return "", errors.New("not found") },
		getwd:               func() (string, error) { return "/tmp", nil },
	}
}

func TestParseMCPStdioCLI(t *testing.T) {
	t.Parallel()
	d := newTestMCPCLIDeps()
	cfg, err := d.parseMCPStdioCLI([]string{"--session", "session-a", "--mcp", "gopls"})
	if err != nil {
		t.Fatalf("parseMCPStdioCLI error = %v", err)
	}
	if cfg.sessionName != "session-a" {
		t.Fatalf("sessionName = %q, want %q", cfg.sessionName, "session-a")
	}
	if cfg.mcpName != "gopls" {
		t.Fatalf("mcpName = %q, want %q", cfg.mcpName, "gopls")
	}
}

func TestParseMCPStdioCLI_DefaultDialTimeout(t *testing.T) {
	t.Parallel()
	d := newTestMCPCLIDeps()
	cfg, err := d.parseMCPStdioCLI([]string{"--session", "session-a", "--mcp", "gopls"})
	if err != nil {
		t.Fatalf("parseMCPStdioCLI error = %v", err)
	}
	if cfg.dialTimeout != defaultMCPStdioDialTimeout {
		t.Fatalf("dialTimeout = %v, want %v", cfg.dialTimeout, defaultMCPStdioDialTimeout)
	}
	if cfg.dialTimeout != 8*time.Second {
		t.Fatalf("dialTimeout = %v, want 8s for retry budget", cfg.dialTimeout)
	}
}

func TestParseMCPStdioCLI_MissingSession(t *testing.T) {
	t.Parallel()
	d := newTestMCPCLIDeps()
	d.resolveSessionByEnv = func() string { return "" }
	d.resolveSessionByCwd = func(_, _ string) (string, error) { return "", errors.New("not found") }
	d.getwd = func() (string, error) { return "/tmp", nil }

	_, err := d.parseMCPStdioCLI([]string{"--mcp", "gopls"})
	if err == nil {
		t.Fatal("parseMCPStdioCLI should fail when --session is missing and all fallbacks fail")
	}
}

func TestParseMCPStdioCLI_FallbackToEnvVar(t *testing.T) {
	t.Parallel()
	d := newTestMCPCLIDeps()
	d.resolveSessionByEnv = func() string { return "env-session" }

	cfg, err := d.parseMCPStdioCLI([]string{"--mcp", "gopls"})
	if err != nil {
		t.Fatalf("parseMCPStdioCLI error = %v", err)
	}
	if cfg.sessionName != "env-session" {
		t.Fatalf("sessionName = %q, want %q", cfg.sessionName, "env-session")
	}
}

func TestParseMCPStdioCLI_FallbackToIPCResolve(t *testing.T) {
	t.Parallel()
	d := newTestMCPCLIDeps()
	d.resolveSessionByEnv = func() string { return "" }
	d.getwd = func() (string, error) { return "/my/repo", nil }
	d.resolveSessionByCwd = func(_, cwd string) (string, error) {
		if cwd != "/my/repo" {
			t.Fatalf("cwd = %q, want %q", cwd, "/my/repo")
		}
		return "ipc-session", nil
	}

	cfg, err := d.parseMCPStdioCLI([]string{"--mcp", "gopls"})
	if err != nil {
		t.Fatalf("parseMCPStdioCLI error = %v", err)
	}
	if cfg.sessionName != "ipc-session" {
		t.Fatalf("sessionName = %q, want %q", cfg.sessionName, "ipc-session")
	}
}

func TestParseMCPStdioCLI_FlagTakesPriority(t *testing.T) {
	t.Parallel()
	d := newTestMCPCLIDeps()
	d.resolveSessionByEnv = func() string { return "env-session" }

	cfg, err := d.parseMCPStdioCLI([]string{"--session", "flag-session", "--mcp", "gopls"})
	if err != nil {
		t.Fatalf("parseMCPStdioCLI error = %v", err)
	}
	if cfg.sessionName != "flag-session" {
		t.Fatalf("sessionName = %q, want %q (flag should take priority over env)", cfg.sessionName, "flag-session")
	}
}

func TestResolveMCPStdioViaIPC(t *testing.T) {
	t.Parallel()
	d := newTestMCPCLIDeps()
	d.pipeName = func() string { return `\\.\pipe\myT-x-explicit` }
	d.sendIPCRequest = func(_ string, req ipc.TmuxRequest) (ipc.TmuxResponse, error) {
		if req.Command != "mcp-resolve-stdio" {
			t.Fatalf("command = %q, want %q", req.Command, "mcp-resolve-stdio")
		}
		return ipc.TmuxResponse{
			ExitCode: 0,
			Stdout:   `{"session_name":"s1","mcp_id":"lsp-gopls","pipe_path":"\\\\.\\pipe\\myT-x-mcp-user-s1-lsp-gopls"}`,
		}, nil
	}
	resolved, err := d.resolveMCPStdioViaIPC("s1", "gopls")
	if err != nil {
		t.Fatalf("resolveMCPStdioViaIPC error = %v", err)
	}
	if resolved.MCPID != "lsp-gopls" {
		t.Fatalf("MCPID = %q, want %q", resolved.MCPID, "lsp-gopls")
	}
	if !strings.Contains(resolved.PipePath, "lsp-gopls") {
		t.Fatalf("PipePath = %q, want lsp-gopls suffix", resolved.PipePath)
	}
}

func TestResolveMCPStdioViaIPC_PassesExplicitPipeName(t *testing.T) {
	t.Parallel()
	d := newTestMCPCLIDeps()
	d.pipeName = func() string { return `\\.\pipe\myT-x-explicit` }
	d.sendIPCRequest = func(pipeName string, req ipc.TmuxRequest) (ipc.TmuxResponse, error) {
		if pipeName != `\\.\pipe\myT-x-explicit` {
			t.Fatalf("pipeName = %q, want explicit default pipe name", pipeName)
		}
		return ipc.TmuxResponse{
			ExitCode: 0,
			Stdout:   `{"session_name":"s1","mcp_id":"lsp-gopls","pipe_path":"\\\\.\\pipe\\myT-x-mcp-user-s1-lsp-gopls"}`,
		}, nil
	}
	if _, err := d.resolveMCPStdioViaIPC("s1", "gopls"); err != nil {
		t.Fatalf("resolveMCPStdioViaIPC error = %v", err)
	}
}

func TestResolveMCPStdioViaIPC_RejectsMismatchedSessionName(t *testing.T) {
	t.Parallel()
	d := newTestMCPCLIDeps()
	d.sendIPCRequest = func(_ string, req ipc.TmuxRequest) (ipc.TmuxResponse, error) {
		return ipc.TmuxResponse{
			ExitCode: 0,
			Stdout:   `{"session_name":"other","mcp_id":"lsp-gopls","pipe_path":"\\\\.\\pipe\\myT-x-mcp-user-other-lsp-gopls"}`,
		}, nil
	}
	_, err := d.resolveMCPStdioViaIPC("s1", "gopls")
	if err == nil {
		t.Fatal("resolveMCPStdioViaIPC should fail for mismatched session_name")
	}
	if !strings.Contains(err.Error(), `want "s1"`) {
		t.Fatalf("error = %v, want session_name mismatch", err)
	}
}

func TestResolveMCPStdioViaIPC_AllowsTrimmedCaseInsensitiveSessionName(t *testing.T) {
	t.Parallel()
	d := newTestMCPCLIDeps()
	d.sendIPCRequest = func(_ string, req ipc.TmuxRequest) (ipc.TmuxResponse, error) {
		return ipc.TmuxResponse{
			ExitCode: 0,
			Stdout:   `{"session_name":" S1 ","mcp_id":"lsp-gopls","pipe_path":"\\\\.\\pipe\\myT-x-mcp-user-s1-lsp-gopls"}`,
		}, nil
	}
	resolved, err := d.resolveMCPStdioViaIPC("s1", "gopls")
	if err != nil {
		t.Fatalf("resolveMCPStdioViaIPC error = %v", err)
	}
	if resolved.SessionName != " S1 " {
		t.Fatalf("SessionName = %q, want original payload value", resolved.SessionName)
	}
}

func TestResolveMCPStdioViaIPC_ConnectionErrorShowsGUIHint(t *testing.T) {
	t.Parallel()
	callCount := 0
	d := newTestMCPCLIDeps()
	d.sendIPCRequest = func(string, ipc.TmuxRequest) (ipc.TmuxResponse, error) {
		callCount++
		return ipc.TmuxResponse{}, &net.OpError{Op: "dial", Err: errors.New("pipe unavailable")}
	}

	_, err := d.resolveMCPStdioViaIPC("s1", "gopls")
	if err == nil {
		t.Fatal("resolveMCPStdioViaIPC should fail when IPC is unavailable")
	}
	if !strings.Contains(err.Error(), "myT-x IPC is unavailable") {
		t.Fatalf("error = %v, want GUI startup hint", err)
	}
	if callCount != ipcResolveMaxRetries {
		t.Fatalf("sendIPCRequest called %d times, want %d (all retries exhausted)", callCount, ipcResolveMaxRetries)
	}
}

func TestResolveMCPStdioViaIPC_ConnectionErrorRetriesAndSucceeds(t *testing.T) {
	t.Parallel()
	callCount := 0
	d := newTestMCPCLIDeps()
	d.sendIPCRequest = func(_ string, req ipc.TmuxRequest) (ipc.TmuxResponse, error) {
		callCount++
		if callCount <= 2 {
			return ipc.TmuxResponse{}, &net.OpError{Op: "dial", Err: errors.New("pipe unavailable")}
		}
		payload := `{"session_name":"s1","mcp_id":"gopls","pipe_path":"\\\\.\\pipe\\test"}`
		return ipc.TmuxResponse{ExitCode: 0, Stdout: payload}, nil
	}

	resolved, err := d.resolveMCPStdioViaIPC("s1", "gopls")
	if err != nil {
		t.Fatalf("resolveMCPStdioViaIPC error = %v, want success after retry", err)
	}
	if resolved.PipePath != `\\.\pipe\test` {
		t.Fatalf("PipePath = %q, want %q", resolved.PipePath, `\\.\pipe\test`)
	}
	if callCount != 3 {
		t.Fatalf("sendIPCRequest called %d times, want 3 (2 failures + 1 success)", callCount)
	}
}

func TestResolveMCPStdioViaIPC_NonConnectionErrorDoesNotRetry(t *testing.T) {
	t.Parallel()
	callCount := 0
	d := newTestMCPCLIDeps()
	d.sendIPCRequest = func(string, ipc.TmuxRequest) (ipc.TmuxResponse, error) {
		callCount++
		return ipc.TmuxResponse{}, errors.New("protocol error")
	}

	_, err := d.resolveMCPStdioViaIPC("s1", "gopls")
	if err == nil {
		t.Fatal("resolveMCPStdioViaIPC should fail for non-connection errors")
	}
	if !strings.Contains(err.Error(), "ipc request failed") {
		t.Fatalf("error = %v, want ipc request failed error", err)
	}
	if callCount != 1 {
		t.Fatalf("sendIPCRequest called %d times, want 1 (no retry for non-connection errors)", callCount)
	}
}

func TestResolveMCPStdioViaIPC_InvalidJSONPayload(t *testing.T) {
	t.Parallel()
	d := newTestMCPCLIDeps()
	d.sendIPCRequest = func(string, ipc.TmuxRequest) (ipc.TmuxResponse, error) {
		return ipc.TmuxResponse{
			ExitCode: 0,
			Stdout:   "{invalid json",
		}, nil
	}

	_, err := d.resolveMCPStdioViaIPC("s1", "gopls")
	if err == nil {
		t.Fatal("resolveMCPStdioViaIPC should fail for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse ipc payload") {
		t.Fatalf("error = %v, want parse ipc payload error", err)
	}
}

func TestResolveMCPStdioViaIPC_EmptyStderrFallsBackToUnknownError(t *testing.T) {
	t.Parallel()
	d := newTestMCPCLIDeps()
	d.sendIPCRequest = func(string, ipc.TmuxRequest) (ipc.TmuxResponse, error) {
		return ipc.TmuxResponse{
			ExitCode: 1,
			Stderr:   "   ",
		}, nil
	}

	_, err := d.resolveMCPStdioViaIPC("s1", "gopls")
	if err == nil {
		t.Fatal("resolveMCPStdioViaIPC should fail for non-zero exit code")
	}
	if err.Error() != "unknown error" {
		t.Fatalf("error = %q, want %q", err.Error(), "unknown error")
	}
}

func TestExecuteMCPCLI_RejectsUnsupportedPlatformBeforeIPC(t *testing.T) {
	t.Parallel()
	d := newTestMCPCLIDeps()
	d.platformSupported = func() bool { return false }
	d.sendIPCRequest = func(string, ipc.TmuxRequest) (ipc.TmuxResponse, error) {
		t.Fatal("sendIPCRequest should not be called on unsupported platforms")
		return ipc.TmuxResponse{}, nil
	}

	var stderr bytes.Buffer
	exitCode := d.executeMCPCLIWith(
		[]string{"stdio", "--session", "s1", "--mcp", "gopls"},
		strings.NewReader(""),
		io.Discard,
		&stderr,
	)
	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", exitCode)
	}
	if !strings.Contains(stderr.String(), "supported only on Windows") {
		t.Fatalf("stderr = %q, want unsupported platform message", stderr.String())
	}
}

// ---------------------------------------------------------------------------
// Field count guard tests
// ---------------------------------------------------------------------------

func TestMCPCLIDepsFieldCount(t *testing.T) {
	t.Parallel()
	const expectedFields = 6
	actual := reflect.TypeFor[mcpCLIDeps]().NumField()
	if actual != expectedFields {
		t.Fatalf("mcpCLIDeps has %d fields, expected %d — update newTestMCPCLIDeps and defaultMCPCLIDeps for new fields", actual, expectedFields)
	}
}

func TestDefaultMCPCLIDepsAllFieldsNonNil(t *testing.T) {
	t.Parallel()
	d := defaultMCPCLIDeps()
	v := reflect.ValueOf(d)
	for i := range v.NumField() {
		f := v.Field(i)
		if f.Kind() == reflect.Func && f.IsNil() {
			t.Errorf("defaultMCPCLIDeps().%s is nil", v.Type().Field(i).Name)
		}
	}
}
