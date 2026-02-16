package tmux

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"myT-x/internal/ipc"
)

// EventEmitter emits backend events to any host (Wails, logger, etc).
type EventEmitter interface {
	Emit(name string, payload any)
}

// EventEmitterFunc adapts a function into EventEmitter.
type EventEmitterFunc func(name string, payload any)

func (f EventEmitterFunc) Emit(name string, payload any) {
	f(name, payload)
}

type noopEmitter struct{}

func (noopEmitter) Emit(string, any) {}

// RouterOptions controls command router behavior.
type RouterOptions struct {
	DefaultShell  string
	PipeName      string
	HostPID       int
	ShimAvailable bool              // true when tmux CLI shim is installed on PATH
	PaneEnv       map[string]string // default env vars; protected by paneEnvMu, updated via UpdatePaneEnv()
}

// CommandRouter dispatches tmux-compatible commands.
type CommandRouter struct {
	// shimMu guards opts.ShimAvailable only.
	// paneEnvMu guards opts.PaneEnv only.
	// shimMu and paneEnvMu are independent — never held simultaneously.
	shimMu    sync.RWMutex
	paneEnvMu sync.RWMutex
	sessions  *SessionManager
	emitter   EventEmitter
	opts      RouterOptions
	handlers  map[string]func(ipc.TmuxRequest) ipc.TmuxResponse
}

// PipeName returns the configured IPC pipe name.
func (r *CommandRouter) PipeName() string {
	return r.opts.PipeName
}

// ShimAvailable reports whether the tmux CLI shim is installed.
func (r *CommandRouter) ShimAvailable() bool {
	r.shimMu.RLock()
	defer r.shimMu.RUnlock()
	return r.opts.ShimAvailable
}

// SetShimAvailable updates the shim availability flag at runtime.
func (r *CommandRouter) SetShimAvailable(available bool) {
	r.shimMu.Lock()
	r.opts.ShimAvailable = available
	r.shimMu.Unlock()
	slog.Debug("[agent-teams] ShimAvailable updated", "available", available)
}

// UpdatePaneEnv replaces PaneEnv at runtime (called after SaveConfig).
// The provided map is deep-copied to avoid shared references.
func (r *CommandRouter) UpdatePaneEnv(paneEnv map[string]string) {
	copied := make(map[string]string, len(paneEnv))
	for k, v := range paneEnv {
		copied[k] = v
	}
	r.paneEnvMu.Lock()
	r.opts.PaneEnv = copied
	r.paneEnvMu.Unlock()
	slog.Debug("[DEBUG-ROUTER] PaneEnv updated", "count", len(copied))
}

// PaneEnvSnapshot returns a snapshot (deep copy) of current PaneEnv.
// Exported for cross-package verification (e.g. app-layer tests).
func (r *CommandRouter) PaneEnvSnapshot() map[string]string {
	return r.getPaneEnv()
}

// paneEnvView returns a read-only map reference of current PaneEnv.
// Callers must not mutate the returned map.
// NOTE: safety relies on UpdatePaneEnv using copy-on-write replacement
// (allocate a new map, then swap under paneEnvMu) rather than in-place writes.
// See UpdatePaneEnv for the writer side of this contract.
func (r *CommandRouter) paneEnvView() map[string]string {
	r.paneEnvMu.RLock()
	defer r.paneEnvMu.RUnlock()
	return r.opts.PaneEnv
}

// getPaneEnv returns a snapshot (deep copy) of current PaneEnv.
func (r *CommandRouter) getPaneEnv() map[string]string {
	r.paneEnvMu.RLock()
	defer r.paneEnvMu.RUnlock()
	src := r.opts.PaneEnv
	if len(src) == 0 {
		return nil
	}
	copied := make(map[string]string, len(src))
	for k, v := range src {
		copied[k] = v
	}
	return copied
}

// NewCommandRouter creates a new command router.
func NewCommandRouter(sessions *SessionManager, emitter EventEmitter, opts RouterOptions) *CommandRouter {
	if sessions == nil {
		sessions = NewSessionManager()
	}
	if emitter == nil {
		emitter = noopEmitter{}
	}
	if opts.PipeName == "" {
		opts.PipeName = ipc.DefaultPipeName()
	}
	if opts.HostPID <= 0 {
		opts.HostPID = os.Getpid()
	}
	router := &CommandRouter{
		sessions: sessions,
		emitter:  emitter,
		opts:     opts,
	}
	router.handlers = map[string]func(ipc.TmuxRequest) ipc.TmuxResponse{
		"new-session":     router.handleNewSession,
		"has-session":     router.handleHasSession,
		"split-window":    router.handleSplitWindow,
		"send-keys":       router.handleSendKeys,
		"select-pane":     router.handleSelectPane,
		"list-sessions":   router.handleListSessions,
		"kill-session":    router.handleKillSession,
		"list-panes":      router.handleListPanes,
		"display-message": router.handleDisplayMessage,
		"activate-window": router.handleActivateWindow,
	}
	return router
}

// Execute handles one tmux request.
func (r *CommandRouter) Execute(req ipc.TmuxRequest) ipc.TmuxResponse {
	req.Command = strings.TrimSpace(req.Command)
	if req.Flags == nil {
		req.Flags = map[string]any{}
	}
	if req.Env == nil {
		req.Env = map[string]string{}
	}

	slog.Debug("[DEBUG-SHIM] Execute",
		"command", req.Command,
		"flags", fmt.Sprintf("%v", req.Flags),
		"args", req.Args,
		"env", req.Env,
		"callerPane", req.CallerPane,
	)

	if handler, ok := r.handlers[req.Command]; ok {
		return handler(req)
	}
	return ipc.TmuxResponse{
		ExitCode: 1,
		Stderr:   fmt.Sprintf("unknown command: %s\n", req.Command),
	}
}
