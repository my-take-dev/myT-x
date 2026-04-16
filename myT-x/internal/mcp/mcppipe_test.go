package mcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os/user"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"myT-x/internal/mcp/pipebridge"
)

func TestBuildMCPPipeName(t *testing.T) {
	tests := []struct {
		session string
		mcpID   string
	}{
		{"session-1", "gopls-lsp"},
		{"my-project", "memory"},
		{"test_session", "ts-lsp"},
	}

	for _, tt := range tests {
		t.Run(tt.session+"_"+tt.mcpID, func(t *testing.T) {
			name := BuildMCPPipeName(tt.session, tt.mcpID)
			if !strings.HasPrefix(name, `\\.\pipe\myT-x-mcp-`) {
				t.Errorf("pipe name should start with myT-x-mcp prefix, got %q", name)
			}
			if !strings.Contains(name, tt.session) {
				t.Errorf("pipe name should contain session name %q, got %q", tt.session, name)
			}
			if !strings.Contains(name, tt.mcpID) {
				t.Errorf("pipe name should contain mcp ID %q, got %q", tt.mcpID, name)
			}
		})
	}
}

func TestBuildMCPPipeName_SanitizesSpecialChars(t *testing.T) {
	name := BuildMCPPipeName("session with spaces", "mcp.id/slash")
	// Spaces, dots, slashes should be replaced with underscores.
	if strings.Contains(name, " ") || strings.Contains(name, "/") {
		t.Errorf("pipe name should not contain spaces or slashes, got %q", name)
	}
}

func TestBuildMCPPipeName_IncludesUsername(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		t.Skip("cannot determine current user")
	}
	username := u.Username
	if idx := strings.LastIndex(username, `\`); idx >= 0 {
		username = username[idx+1:]
	}

	name := BuildMCPPipeName("session", "mcp")
	if !strings.Contains(name, username) {
		t.Errorf("pipe name should contain username %q, got %q", username, name)
	}
}

func TestBuildMCPPipeName_Deterministic(t *testing.T) {
	a := BuildMCPPipeName("session-1", "gopls")
	b := BuildMCPPipeName("session-1", "gopls")
	if a != b {
		t.Errorf("pipe name should be deterministic: %q != %q", a, b)
	}
}

func TestBuildMCPPipeName_UniquePerSessionAndMCP(t *testing.T) {
	a := BuildMCPPipeName("session-1", "gopls")
	b := BuildMCPPipeName("session-2", "gopls")
	c := BuildMCPPipeName("session-1", "tsls")

	if a == b {
		t.Errorf("different sessions should produce different names")
	}
	if a == c {
		t.Errorf("different MCP IDs should produce different names")
	}
}

func TestNewMCPPipeServer_NotStarted(t *testing.T) {
	srv := NewMCPPipeServer(MCPPipeConfig{
		PipeName:   `\\.\pipe\test-mcp-not-started`,
		LSPCommand: "gopls",
		RootDir:    ".",
	})

	if srv.PipeName() != `\\.\pipe\test-mcp-not-started` {
		t.Errorf("PipeName() = %q, want %q", srv.PipeName(), `\\.\pipe\test-mcp-not-started`)
	}

	// Stop on a never-started server should be safe (no-op).
	srv.Stop()
}

func TestMCPPipeServer_StartStop(t *testing.T) {
	pipeName := fmt.Sprintf(`\\.\pipe\test-mcp-start-stop-%d`, time.Now().UnixNano())
	srv := NewMCPPipeServer(MCPPipeConfig{
		PipeName:   pipeName,
		LSPCommand: "gopls",
		RootDir:    t.TempDir(),
	})

	if err := srv.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Double start should return error.
	if err := srv.Start(); err == nil {
		t.Error("double Start() should return error")
	}

	srv.Stop()

	// Stop is idempotent.
	srv.Stop()
}

func TestMCPPipeSecurityDescriptor(t *testing.T) {
	sd, err := mcpPipeSecurityDescriptor()
	if err != nil {
		t.Fatalf("mcpPipeSecurityDescriptor() error = %v", err)
	}
	// Should contain DACL markers.
	if !strings.HasPrefix(sd, "D:P(") {
		t.Errorf("security descriptor should start with D:P(, got %q", sd)
	}
	// Should include SYSTEM ACE.
	if !strings.Contains(sd, "SY") {
		t.Errorf("security descriptor should include SYSTEM (SY), got %q", sd)
	}
}

func TestNewCloseOnce_CallsCloserOnlyOnce(t *testing.T) {
	var calls atomic.Int32
	closeOnce := newCloseOnce(func() error {
		calls.Add(1)
		return nil
	})

	if err := closeOnce(); err != nil {
		t.Fatalf("first closeOnce call error = %v", err)
	}
	if err := closeOnce(); err != nil {
		t.Fatalf("second closeOnce call error = %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("closeOnce call count = %d, want 1", got)
	}
}

func TestNewCloseOnce_ReturnsOriginalCloseError(t *testing.T) {
	closeErr := errors.New("close failed")
	closeOnce := newCloseOnce(func() error {
		return closeErr
	})

	if err := closeOnce(); !errors.Is(err, closeErr) {
		t.Fatalf("first closeOnce error = %v, want %v", err, closeErr)
	}
	if err := closeOnce(); !errors.Is(err, closeErr) {
		t.Fatalf("second closeOnce error = %v, want %v", err, closeErr)
	}
}

func TestMCPPipeServerFieldCount(t *testing.T) {
	got := reflect.TypeFor[MCPPipeServer]().NumField()
	want := 8
	if got != want {
		t.Fatalf("MCPPipeServer field count = %d, want %d", got, want)
	}
}

func TestMCPPipeConfigFieldCount(t *testing.T) {
	got := reflect.TypeFor[MCPPipeConfig]().NumField()
	want := 5
	if got != want {
		t.Fatalf("MCPPipeConfig field count = %d, want %d", got, want)
	}
}

type testMetadataRuntime struct {
	startCallerPane string
}

func (r *testMetadataRuntime) Start(ctx context.Context) error {
	r.startCallerPane = pipebridge.CallerPaneIDFromContext(ctx)
	return nil
}

func (*testMetadataRuntime) Serve(context.Context) error { return nil }

func (*testMetadataRuntime) Close(context.Context) error { return nil }

func TestMCPPipeServerHandleConnectionPropagatesCallerPane(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	runtime := &testMetadataRuntime{}
	srv := NewMCPPipeServer(MCPPipeConfig{
		PipeName: "test-pipe",
		RuntimeFactory: func(in io.Reader, out io.Writer) (MCPRuntime, error) {
			return runtime, nil
		},
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.handleConnection(serverConn)
	}()

	if err := pipebridge.WriteCallerPaneHandshake(clientConn, "%7"); err != nil {
		t.Fatalf("WriteCallerPaneHandshake: %v", err)
	}
	if err := clientConn.Close(); err != nil {
		t.Fatalf("clientConn.Close: %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handleConnection did not finish")
	}

	if runtime.startCallerPane != "%7" {
		t.Fatalf("start caller pane = %q, want %%7", runtime.startCallerPane)
	}
}

func TestMCPPipeServerHandleConnectionStopsDuringHandshake(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	srv := NewMCPPipeServer(MCPPipeConfig{PipeName: "test-pipe"})

	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.handleConnection(serverConn)
	}()

	srv.cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handleConnection did not stop during handshake")
	}
}
