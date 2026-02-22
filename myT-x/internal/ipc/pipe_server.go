package ipc

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os/user"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Microsoft/go-winio"
)

const (
	defaultPipeConnTimeout              = 30 * time.Second
	maxPipeRequestBytes                 = 64 * 1024 // limits request size to prevent memory exhaustion
	defaultPipeMaxConcurrentConnections = 64
	connSlotAcquireTimeout              = 5 * time.Second
)

// PipeServer receives requests from tmux shim clients over Named Pipe.
type PipeServer struct {
	pipeName string
	router   CommandExecutor

	ctx    context.Context
	cancel context.CancelFunc

	mu        sync.Mutex
	listener  net.Listener
	started   bool
	wg        sync.WaitGroup
	connSlots chan struct{}
}

// NewPipeServer constructs a PipeServer.
func NewPipeServer(pipeName string, router CommandExecutor) *PipeServer {
	ctx, cancel := context.WithCancel(context.Background())
	if pipeName == "" {
		pipeName = DefaultPipeName()
	}
	return &PipeServer{
		pipeName:  pipeName,
		router:    router,
		ctx:       ctx,
		cancel:    cancel,
		connSlots: make(chan struct{}, defaultPipeMaxConcurrentConnections),
	}
}

// PipeName returns the listen pipe name.
func (s *PipeServer) PipeName() string {
	return s.pipeName
}

// Start begins listening on the Named Pipe.
func (s *PipeServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return errors.New("pipe server already started")
	}
	if s.router == nil {
		return errors.New("pipe server requires router")
	}

	listener, err := listenPipeWithCurrentUserDACL(s.pipeName)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.pipeName, err)
	}

	s.listener = listener
	s.started = true
	s.wg.Go(s.acceptLoop)
	return nil
}

// Stop gracefully shuts down the server.
func (s *PipeServer) Stop() error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	s.started = false
	s.cancel()
	listener := s.listener
	s.listener = nil
	s.mu.Unlock()

	if listener != nil {
		if err := listener.Close(); err != nil {
			slog.Warn("[ipc] failed to close pipe listener during shutdown", "error", err)
		}
	}
	s.wg.Wait()
	return nil
}

func (s *PipeServer) acceptLoop() {
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
					slog.Warn("[ipc] accept loop: repeated failures, possible permanent error", "error", err, "count", consecutiveErrors)
					time.Sleep(500 * time.Millisecond)
				} else {
					slog.Debug("[ipc] accept error", "error", err)
				}
				continue
			}
		}
		consecutiveErrors = 0

		if !s.acquireConnectionSlot() {
			s.writeResponse(conn, TmuxResponse{
				ExitCode: 1,
				Stderr:   "server busy, try again later\n",
			})
			if closeErr := conn.Close(); closeErr != nil {
				slog.Debug("[ipc] failed to close rejected connection", "error", closeErr)
			}
			continue
		}

		// Go 1.22+: for-loop variables are scoped per iteration, so conn
		// refers to the specific connection accepted in this iteration.
		// Each goroutine captures a distinct conn value and does not share
		// it with other iterations.
		s.wg.Go(func() {
			defer s.releaseConnectionSlot()
			s.handleConnection(conn)
		})
	}
}

// handleConnection processes a single client connection (one command per connection).
// A deadline of defaultPipeConnTimeout is enforced and requests exceeding
// maxPipeRequestBytes are rejected with an error response.
func (s *PipeServer) handleConnection(conn net.Conn) {
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(defaultPipeConnTimeout)); err != nil {
		slog.Warn("[ipc] failed to set connection deadline", "error", err)
		return
	}

	reader := bufio.NewReaderSize(conn, maxPipeRequestBytes+1)
	rawReq, err := readRequestFrame(reader)
	if errors.Is(err, io.EOF) {
		slog.Debug("[ipc] client disconnected without sending data")
		return
	}
	if err != nil {
		s.writeResponse(conn, TmuxResponse{
			ExitCode: 1,
			Stderr:   fmt.Sprintf("invalid request: %v\n", err),
		})
		return
	}

	req, err := decodeRequest(rawReq)
	if err != nil {
		s.writeResponse(conn, TmuxResponse{
			ExitCode: 1,
			Stderr:   fmt.Sprintf("invalid request: %v\n", err),
		})
		return
	}

	slog.Debug("[DEBUG-IPC-PIPE] received request from shim",
		"command", req.Command,
		"callerPane", req.CallerPane,
		"args", fmt.Sprintf("%v", req.Args),
		"flags", fmt.Sprintf("%v", req.Flags),
	)

	resp := s.router.Execute(req)
	s.writeResponse(conn, resp)
}

func (s *PipeServer) writeResponse(conn net.Conn, resp TmuxResponse) {
	rawResp, err := encodeResponse(resp)
	if err != nil {
		slog.Warn("[ipc] failed to encode response", "error", err, "exitCode", resp.ExitCode)
		rawResp = []byte(`{"exit_code":1,"stderr":"internal encode error\n"}`)
	}
	if _, err := conn.Write(rawResp); err != nil {
		slog.Debug("[ipc] failed to write response", "error", err)
		return
	}
	if _, err := conn.Write([]byte{'\n'}); err != nil {
		slog.Debug("[ipc] failed to write response delimiter", "error", err)
	}
}

func readRequestFrame(reader *bufio.Reader) ([]byte, error) {
	raw, err := reader.ReadSlice('\n')
	if errors.Is(err, bufio.ErrBufferFull) {
		return nil, fmt.Errorf("request exceeds %d bytes", maxPipeRequestBytes)
	}
	if errors.Is(err, io.EOF) {
		if len(raw) == 0 {
			return nil, io.EOF
		}
		return raw, nil
	}
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func (s *PipeServer) acquireConnectionSlot() bool {
	if s.connSlots == nil {
		return true
	}
	timer := time.NewTimer(connSlotAcquireTimeout)
	defer timer.Stop()
	select {
	case s.connSlots <- struct{}{}:
		return true
	case <-timer.C:
		slog.Warn("[ipc] connection slot exhausted, rejecting client")
		return false
	case <-s.ctx.Done():
		return false
	}
}

func (s *PipeServer) releaseConnectionSlot() {
	if s.connSlots == nil {
		return
	}
	select {
	case <-s.connSlots:
	default:
		slog.Warn("[ipc] releaseConnectionSlot: no slot to release (possible double-release)")
	}
}

// listenPipeWithCurrentUserDACL creates a Named Pipe listener restricted to the
// current user. The DACL grants full access only to SYSTEM and the current
// user's SID, preventing other local users from connecting.
func listenPipeWithCurrentUserDACL(pipeName string) (net.Listener, error) {
	securityDescriptor, err := pipeSecurityDescriptor()
	if err != nil {
		return nil, err
	}
	return winio.ListenPipe(pipeName, &winio.PipeConfig{
		SecurityDescriptor: securityDescriptor,
		MessageMode:        false,
		InputBufferSize:    int32(maxPipeRequestBytes),
		OutputBufferSize:   int32(maxPipeResponseBytes),
	})
}

var validSIDPattern = regexp.MustCompile(`^S-1(-\d+)+$`)

func pipeSecurityDescriptor() (string, error) {
	current, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("resolve current user: %w", err)
	}
	sid := strings.TrimSpace(current.Uid)
	if sid == "" {
		return "", errors.New("current user SID is unavailable")
	}
	if !validSIDPattern.MatchString(sid) {
		return "", fmt.Errorf("current user SID has unexpected format: %s", sid)
	}
	// SDDL: D:P = protected DACL (no inheritance)
	// (A;;GA;;;SY) = full access for SYSTEM
	// (A;;GA;;;%s) = full access for current user SID
	return fmt.Sprintf("D:P(A;;GA;;;SY)(A;;GA;;;%s)", sid), nil
}
