package singletaskrunnermcp

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"sync"

	"myT-x/internal/mcp/single-task-runner/adapter/mcptool"
	internalmcp "myT-x/internal/mcp/single-task-runner/internal/mcp"
	"myT-x/internal/singletaskrunner"
)

const (
	// DefaultServerName is used when Config.ServerName is empty.
	DefaultServerName = "single-task-runner"
	// DefaultServerVersion is used when Config.ServerVersion is empty.
	DefaultServerVersion = "0.1.0"
)

// Config configures the embedded MCP runtime.
type Config struct {
	In  io.Reader
	Out io.Writer

	Logger *log.Logger

	Service *singletaskrunner.Service

	ServerName    string
	ServerVersion string
}

// App is the embeddable runtime interface.
type App interface {
	Start(ctx context.Context) error
	Serve(ctx context.Context) error
	Close(ctx context.Context) error
}

// Runtime connects the single-task-runner service to the MCP stdio server.
type Runtime struct {
	cfg    Config
	server *internalmcp.Server

	mu      sync.Mutex
	started bool
	closed  bool
}

// NewApp creates an embeddable runtime.
func NewApp(cfg Config) (App, error) {
	return NewRuntime(cfg)
}

// NewRuntime creates an embeddable runtime.
func NewRuntime(cfg Config) (*Runtime, error) {
	normalized, err := normalizeConfig(cfg)
	if err != nil {
		return nil, err
	}

	handler := mcptool.NewHandler(normalized.Service)
	registry, err := handler.BuildRegistry()
	if err != nil {
		return nil, err
	}

	server := internalmcp.NewServer(internalmcp.Config{
		Name:     normalized.ServerName,
		Version:  normalized.ServerVersion,
		In:       normalized.In,
		Out:      normalized.Out,
		Logger:   normalized.Logger,
		Registry: registry,
	})

	return &Runtime{
		cfg:    normalized,
		server: server,
	}, nil
}

// Run is the one-shot runtime entrypoint.
func Run(ctx context.Context, cfg Config) error {
	runtime, err := NewRuntime(cfg)
	if err != nil {
		return err
	}
	if err := runtime.Start(ctx); err != nil {
		return err
	}
	defer func() {
		if closeErr := runtime.Close(context.Background()); closeErr != nil && runtime.cfg.Logger != nil {
			runtime.cfg.Logger.Printf("runtime close failed: %v", closeErr)
		}
	}()
	return runtime.Serve(ctx)
}

// Start initializes the runtime.
func (r *Runtime) Start(_ context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return errors.New("runtime is closed")
	}
	if r.started {
		return nil
	}

	r.started = true
	return nil
}

// Serve blocks and processes MCP requests.
func (r *Runtime) Serve(ctx context.Context) error {
	if err := r.Start(ctx); err != nil {
		return err
	}
	return r.server.Serve(ctx)
}

// Close closes the runtime and interrupts any in-progress Serve call.
func (r *Runtime) Close(_ context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}
	r.closed = true
	r.started = false

	// Signal the server to stop and unblock the Serve loop's blocking read.
	r.server.Shutdown()
	return nil
}

func normalizeConfig(cfg Config) (Config, error) {
	if cfg.Service == nil {
		return Config{}, errors.New("service is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = log.New(io.Discard, "", 0)
	}
	if cfg.In == nil {
		cfg.Logger.Printf("WARNING: In is nil, falling back to os.Stdin")
		cfg.In = os.Stdin
	}
	if cfg.Out == nil {
		cfg.Logger.Printf("WARNING: Out is nil, falling back to os.Stdout")
		cfg.Out = os.Stdout
	}
	if cfg.ServerName == "" {
		cfg.ServerName = DefaultServerName
	}
	if cfg.ServerVersion == "" {
		cfg.ServerVersion = DefaultServerVersion
	}
	return cfg, nil
}
