package mcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os/user"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Microsoft/go-winio"

	"myT-x/internal/mcp/lspmcp"
)

const (
	mcpPipeMaxConcurrentConnections = 8
	mcpPipeConnSlotTimeout          = 5 * time.Second
	mcpPipeRuntimeCloseTimeout      = 5 * time.Second
	mcpPipeInputBufferSize          = 64 * 1024
	mcpPipeOutputBufferSize         = 64 * 1024
)

// MCPPipeConfig holds the configuration for an MCP Named Pipe server.
type MCPPipeConfig struct {
	// PipeName is the full Named Pipe path (e.g. \\.\pipe\myT-x-mcp-...).
	PipeName string
	// LSPCommand is the LSP server binary to launch (e.g. "gopls").
	// Used when RuntimeFactory is nil (legacy LSP path).
	LSPCommand string
	// LSPArgs are additional arguments passed to the LSP server.
	LSPArgs []string
	// RootDir is the workspace root directory for LSP initialization.
	RootDir string
	// RuntimeFactory, when set, is used instead of the default lspmcp.NewRuntime
	// path. This allows non-LSP runtimes (e.g. agent-orchestrator) to share the
	// same pipe server infrastructure.
	RuntimeFactory RuntimeFactory
}

// MCPPipeServer manages a Named Pipe that accepts MCP client connections.
// Each accepted connection creates an independent lspmcp.Runtime that
// bridges the Named Pipe I/O to a dedicated LSP subprocess.
//
// Thread safety: all exported methods are safe for concurrent use.
type MCPPipeServer struct {
	cfg       MCPPipeConfig
	listener  net.Listener
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.Mutex
	started   bool
	connSlots chan struct{}
}

// NewMCPPipeServer constructs a pipe server without starting it.
func NewMCPPipeServer(cfg MCPPipeConfig) *MCPPipeServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &MCPPipeServer{
		cfg:       cfg,
		ctx:       ctx,
		cancel:    cancel,
		connSlots: make(chan struct{}, mcpPipeMaxConcurrentConnections),
	}
}

// PipeName returns the Named Pipe path this server listens on.
func (s *MCPPipeServer) PipeName() string {
	return s.cfg.PipeName
}

// Start begins listening on the Named Pipe and accepting connections.
func (s *MCPPipeServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return errors.New("mcp pipe server already started")
	}

	listener, err := listenMCPPipe(s.cfg.PipeName)
	if err != nil {
		return fmt.Errorf("listen mcp pipe %s: %w", s.cfg.PipeName, err)
	}

	s.listener = listener
	s.started = true
	s.wg.Go(s.acceptLoop)
	return nil
}

// Stop gracefully shuts down the server.
// It closes the listener, cancels the context (which terminates all active
// lspmcp.Runtime instances), and waits for all goroutines to finish.
func (s *MCPPipeServer) Stop() {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return
	}
	s.started = false
	s.cancel()
	listener := s.listener
	s.listener = nil
	s.mu.Unlock()

	if listener != nil {
		if err := listener.Close(); err != nil {
			slog.Warn("[WARN-MCP-PIPE] failed to close pipe listener", "error", err)
		}
	}
	s.wg.Wait()
}

func (s *MCPPipeServer) acceptLoop() {
	consecutiveErrors := 0
	for {
		s.mu.Lock()
		listener := s.listener
		s.mu.Unlock()
		if listener == nil {
			return
		}

		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				consecutiveErrors++
				if consecutiveErrors > 10 {
					slog.Warn("[WARN-MCP-PIPE] accept: repeated failures",
						"error", err, "count", consecutiveErrors)
					time.Sleep(500 * time.Millisecond)
				} else {
					slog.Debug("[DEBUG-MCP-PIPE] accept error", "error", err)
				}
				continue
			}
		}
		consecutiveErrors = 0

		if !s.acquireConnSlot() {
			slog.Warn("[WARN-MCP-PIPE] connection rejected: slot exhausted")
			if closeErr := conn.Close(); closeErr != nil {
				slog.Debug("[DEBUG-MCP-PIPE] failed to close rejected conn", "error", closeErr)
			}
			continue
		}

		s.wg.Go(func() {
			defer s.releaseConnSlot()
			s.handleConnection(conn)
		})
	}
}

// handleConnection creates an MCPRuntime for the accepted connection
// and serves MCP requests until the connection closes or context is cancelled.
//
// When RuntimeFactory is configured, it is used to create the runtime instead
// of the default lspmcp path. This enables non-LSP runtimes (e.g. orchestrator)
// to share the pipe server infrastructure.
func (s *MCPPipeServer) handleConnection(conn net.Conn) {
	closeConn := newCloseOnce(conn.Close)
	defer func() {
		_ = closeConn()
	}()

	// Ensure a blocking MCP read is interrupted when the server is stopped.
	stopCloseOnCancel := context.AfterFunc(s.ctx, func() {
		if err := closeConn(); err != nil && !errors.Is(err, net.ErrClosed) {
			slog.Debug("[DEBUG-MCP-PIPE] close conn on cancel failed", "error", err)
		}
	})
	defer stopCloseOnCancel()

	slog.Debug("[DEBUG-MCP-PIPE] new connection accepted",
		"pipe", s.cfg.PipeName,
		"lsp", s.cfg.LSPCommand,
		"root", s.cfg.RootDir,
	)

	var runtime MCPRuntime
	var err error
	if s.cfg.RuntimeFactory != nil {
		runtime, err = s.cfg.RuntimeFactory(conn, conn)
	} else {
		runtime, err = lspmcp.NewRuntime(lspmcp.Config{
			LSPCommand: s.cfg.LSPCommand,
			LSPArgs:    append([]string(nil), s.cfg.LSPArgs...),
			RootDir:    s.cfg.RootDir,
			In:         conn,
			Out:        conn,
		})
	}
	if err != nil {
		slog.Warn("[WARN-MCP-PIPE] failed to create runtime",
			"error", err, "lsp", s.cfg.LSPCommand)
		return
	}

	if err := runtime.Start(s.ctx); err != nil {
		slog.Warn("[WARN-MCP-PIPE] failed to start runtime",
			"error", err, "lsp", s.cfg.LSPCommand)
		closeCtx, closeCancel := context.WithTimeout(context.Background(), mcpPipeRuntimeCloseTimeout)
		if closeErr := runtime.Close(closeCtx); closeErr != nil {
			slog.Warn("[WARN-MCP-PIPE] runtime close after start failure",
				"closeError", closeErr,
				"startError", err,
				"lsp", s.cfg.LSPCommand,
			)
		}
		closeCancel()
		return
	}

	// Serve blocks until the connection is closed or ctx is cancelled.
	if err := runtime.Serve(s.ctx); err != nil && !errors.Is(err, context.Canceled) {
		slog.Debug("[DEBUG-MCP-PIPE] serve ended", "error", err)
	}

	closeCtx, closeCancel := context.WithTimeout(context.Background(), mcpPipeRuntimeCloseTimeout)
	defer closeCancel()
	if err := runtime.Close(closeCtx); err != nil {
		slog.Debug("[DEBUG-MCP-PIPE] runtime close error", "error", err)
	}

	slog.Debug("[DEBUG-MCP-PIPE] connection closed",
		"pipe", s.cfg.PipeName, "lsp", s.cfg.LSPCommand)
}

func newCloseOnce(closeFn func() error) func() error {
	var (
		once     sync.Once
		closeErr error
	)
	return func() error {
		once.Do(func() {
			closeErr = closeFn()
		})
		return closeErr
	}
}

func (s *MCPPipeServer) acquireConnSlot() bool {
	timer := time.NewTimer(mcpPipeConnSlotTimeout)
	defer timer.Stop()
	select {
	case s.connSlots <- struct{}{}:
		return true
	case <-timer.C:
		return false
	case <-s.ctx.Done():
		return false
	}
}

func (s *MCPPipeServer) releaseConnSlot() {
	select {
	case <-s.connSlots:
	default:
		slog.Warn("[WARN-MCP-PIPE] releaseConnSlot: no slot to release")
	}
}

// BuildMCPPipeName constructs a session- and MCP-scoped Named Pipe path.
// Format: \\.\pipe\myT-x-mcp-{username}-{sessionName}-{mcpID}
func BuildMCPPipeName(sessionName, mcpID string) string {
	username := "unknown"
	if u, err := user.Current(); err == nil {
		username = u.Username
	}
	// Extract just the username part (remove domain prefix like DOMAIN\user).
	if idx := strings.LastIndex(username, `\`); idx >= 0 {
		username = username[idx+1:]
	}
	// Sanitize for pipe name: replace non-alphanumeric with underscore.
	sanitize := func(s string) string {
		var b strings.Builder
		for _, r := range s {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
				b.WriteRune(r)
			} else {
				b.WriteRune('_')
			}
		}
		return b.String()
	}
	return fmt.Sprintf(`\\.\pipe\myT-x-mcp-%s-%s-%s`,
		sanitize(username), sanitize(sessionName), sanitize(mcpID))
}

// listenMCPPipe creates a Named Pipe listener restricted to the current user.
// Same DACL security as ipc.PipeServer.
func listenMCPPipe(pipeName string) (net.Listener, error) {
	sd, err := mcpPipeSecurityDescriptor()
	if err != nil {
		return nil, err
	}
	return winio.ListenPipe(pipeName, &winio.PipeConfig{
		SecurityDescriptor: sd,
		MessageMode:        false,
		InputBufferSize:    int32(mcpPipeInputBufferSize),
		OutputBufferSize:   int32(mcpPipeOutputBufferSize),
	})
}

var validMCPPipeSIDPattern = regexp.MustCompile(`^S-1(-\d+)+$`)

func mcpPipeSecurityDescriptor() (string, error) {
	current, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("resolve current user: %w", err)
	}
	sid := strings.TrimSpace(current.Uid)
	if sid == "" {
		return "", errors.New("current user SID is unavailable")
	}
	if !validMCPPipeSIDPattern.MatchString(sid) {
		return "", fmt.Errorf("current user SID has unexpected format: %s", sid)
	}
	return fmt.Sprintf("D:P(A;;GA;;;SY)(A;;GA;;;%s)", sid), nil
}
