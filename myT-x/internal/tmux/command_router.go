package tmux

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"strings"
	"sync"

	"myT-x/internal/apptypes"
	"myT-x/internal/ipc"
)

// DefaultTerminalCols is the default terminal width when no explicit size is provided.
const DefaultTerminalCols = 120

// DefaultTerminalRows is the default terminal height when no explicit size is provided.
const DefaultTerminalRows = 40

// EventEmitter is an alias for apptypes.RuntimeEventEmitter.
// All layers share the single RuntimeEventEmitter interface defined in apptypes.
type EventEmitter = apptypes.RuntimeEventEmitter

// MCPStdioResolution reuses the shared IPC payload shape for stdio clients.
type MCPStdioResolution = ipc.MCPStdioResolvePayload

// RouterOptions controls command router behavior.
type RouterOptions struct {
	DefaultShell  string
	PipeName      string
	HostPID       int
	ShimAvailable bool              // true when tmux CLI shim is installed on PATH
	PaneEnv       map[string]string // default env vars; protected by paneEnvMu, updated via UpdatePaneEnv()
	ClaudeEnv     map[string]string // Claude Code env vars; protected by claudeEnvMu
	// OnSessionDestroyed is called after kill-session succeeds.
	// It runs outside of SessionManager locks.
	OnSessionDestroyed func(sessionName string)
	// OnSessionRenamed is called after rename-session succeeds.
	// It runs outside of SessionManager locks.
	OnSessionRenamed func(oldName, newName string)
	// ResolveMCPStdio resolves and prepares a Named Pipe endpoint for one MCP.
	ResolveMCPStdio func(sessionName, mcpName string) (MCPStdioResolution, error)
	// ResolveSessionByCwd resolves a session name from a working directory path.
	// Used by the MCP bridge CLI to auto-detect the session when --session and
	// $MYTX_SESSION are unavailable.
	ResolveSessionByCwd func(cwd string) (string, error)
}

// CommandRouter dispatches tmux-compatible commands.
type CommandRouter struct {
	// shimMu guards opts.ShimAvailable only.
	// paneEnvMu guards opts.PaneEnv only.
	// claudeEnvMu guards opts.ClaudeEnv only.
	// shimMu, paneEnvMu, and claudeEnvMu are independent — never held simultaneously.
	shimMu      sync.RWMutex
	paneEnvMu   sync.RWMutex
	claudeEnvMu sync.RWMutex
	sessions    *SessionManager
	emitter     EventEmitter
	opts        RouterOptions
	buffers     *BufferStore
	handlers    map[string]func(ipc.TmuxRequest) ipc.TmuxResponse
	// renamePane is a narrow test seam used to force non-fatal rename errors.
	renamePane func(paneID string, title string) (string, error)
	// attachTerminalFn is a test seam for attach/rollback paths.
	attachTerminalFn func(pane *TmuxPane, workDir string, env map[string]string, source *TmuxPane) error
	// getSessionForNewWindowFn is a narrow test seam for handleNewWindow's
	// snapshot-refetch rollback path.
	getSessionForNewWindowFn func(sessionName string) (*TmuxSession, bool)

	// Buffer IO operations (defaults set by initBufferFileOps in NewCommandRouter).
	openLoadBufferFile   func(path string) (loadBufferReadCloser, error)
	openSaveBufferFile   func(path string, flag int, perm os.FileMode) (saveBufferWriteCloser, error)
	removeSaveBufferFile func(string) error
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

// NewCommandRouter creates a new command router.
func NewCommandRouter(sessions *SessionManager, emitter EventEmitter, opts RouterOptions) *CommandRouter {
	if sessions == nil {
		sessions = NewSessionManager()
	}
	if emitter == nil {
		emitter = apptypes.NoopEmitter{}
	}
	if opts.PipeName == "" {
		opts.PipeName = ipc.DefaultPipeName()
	}
	if opts.HostPID <= 0 {
		opts.HostPID = os.Getpid()
	}
	// Deep-copy PaneEnv to prevent shared references with the caller.
	if opts.PaneEnv != nil {
		copied := make(map[string]string, len(opts.PaneEnv))
		maps.Copy(copied, opts.PaneEnv)
		opts.PaneEnv = copied
	}
	// Deep-copy ClaudeEnv to prevent shared references with the caller.
	if opts.ClaudeEnv != nil {
		copied := make(map[string]string, len(opts.ClaudeEnv))
		maps.Copy(copied, opts.ClaudeEnv)
		opts.ClaudeEnv = copied
	}

	router := &CommandRouter{
		sessions: sessions,
		emitter:  emitter,
		opts:     opts,
		buffers:  NewBufferStore(),
	}
	router.renamePane = sessions.RenamePane
	router.attachTerminalFn = router.attachTerminal
	router.getSessionForNewWindowFn = sessions.GetSession
	router.initBufferFileOps()
	router.handlers = map[string]func(ipc.TmuxRequest) ipc.TmuxResponse{
		"new-session":            router.handleNewSession,
		"has-session":            router.handleHasSession,
		"split-window":           router.handleSplitWindow,
		"send-keys":              router.handleSendKeys,
		"select-pane":            router.handleSelectPane,
		"list-sessions":          router.handleListSessions,
		"kill-session":           router.handleKillSession,
		"list-panes":             router.handleListPanes,
		"display-message":        router.handleDisplayMessage,
		"activate-window":        router.handleActivateWindow,
		"attach-session":         router.handleAttachSession,
		"kill-pane":              router.handleKillPane,
		"rename-session":         router.handleRenameSession,
		"resize-pane":            router.handleResizePane,
		"show-environment":       router.handleShowEnvironment,
		"set-environment":        router.handleSetEnvironment,
		"list-windows":           router.handleListWindows,
		"rename-window":          router.handleRenameWindow,
		"new-window":             router.handleNewWindow,
		"kill-window":            router.handleKillWindow,
		"select-window":          router.handleSelectWindow,
		"copy-mode":              router.handleCopyMode,
		"list-buffers":           router.handleListBuffers,
		"set-buffer":             router.handleSetBuffer,
		"paste-buffer":           router.handlePasteBuffer,
		"load-buffer":            router.handleLoadBuffer,
		"save-buffer":            router.handleSaveBuffer,
		"capture-pane":           router.handleCapturePane,
		"run-shell":              router.handleRunShell,
		"if-shell":               router.handleIfShell,
		"mcp-resolve-stdio":      router.handleMCPResolveStdio,
		"resolve-session-by-cwd": router.handleResolveSessionByCwd,
	}
	return router
}

// bestEffortSendKeys translates args via TranslateSendKeys and writes to the pane terminal.
// If appendEnter is true, "Enter" is appended using a defensive copy to avoid mutating the
// caller's backing array. Failures are logged at Warn level but never propagated as errors,
// keeping the tmux-shim contract of forwarding over aborting.
//
// S-22: appendEnter is currently always true at all call sites (split-window,
// new-session, new-window). The parameter is retained for explicitness: each
// caller consciously opts in to the Enter suffix, and a future command may
// need appendEnter=false (e.g., send-keys passthrough without implicit Enter).
func (r *CommandRouter) bestEffortSendKeys(pane *TmuxPane, args []string, appendEnter bool, debugTag string, sessionName string) {
	if len(args) == 0 {
		return
	}
	if pane == nil {
		slog.Warn("["+debugTag+"] initial send-keys skipped: pane is nil",
			"session", sessionName,
		)
		return
	}
	term := pane.Terminal
	if term == nil {
		slog.Warn("["+debugTag+"] initial send-keys skipped: pane terminal is nil",
			"session", sessionName,
			"paneId", pane.IDString(),
		)
		return
	}

	sendArgs := args
	if appendEnter {
		// Defensive copy: avoid mutating the caller's backing array when args has spare capacity.
		argsWithEnter := make([]string, len(args)+1)
		copy(argsWithEnter, args)
		argsWithEnter[len(args)] = "Enter"
		sendArgs = argsWithEnter
	}

	payload := TranslateSendKeys(sendArgs)
	if err := writeSendKeysPayload(term, payload); err != nil {
		slog.Warn("["+debugTag+"] initial send-keys failed; continuing",
			"session", sessionName,
			"paneId", pane.IDString(),
			"error", err,
		)
	}
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

	// Guard: avoid fmt.Sprintf allocation on the hot path when debug logging
	// is disabled. send-keys is invoked on every keystroke; unguarded Sprintf
	// adds ~200 B/call of unnecessary heap allocation. See checklist #145.
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		slog.Debug("[DEBUG-SHIM] Execute",
			"command", req.Command,
			"flags", fmt.Sprintf("%v", req.Flags),
			"args", req.Args,
			"env", req.Env,
			"callerPane", req.CallerPane,
		)
	}

	if handler, ok := r.handlers[req.Command]; ok {
		return handler(req)
	}
	return ipc.TmuxResponse{
		ExitCode: 1,
		Stderr:   fmt.Sprintf("unknown command: %s\n", req.Command),
	}
}

// ---------------------------------------------------------------------------
// Shared target resolution helpers (used by multiple handler files)
// ---------------------------------------------------------------------------

func (r *CommandRouter) resolveTargetFromRequest(req ipc.TmuxRequest) (*TmuxPane, error) {
	target := mustString(req.Flags["-t"])
	callerPaneID := ParseCallerPane(req.CallerPane)
	return r.sessions.ResolveTarget(target, callerPaneID)
}

// resolveDirectionalPane resolves a pane in the direction specified by -L/-R/-U/-D flags.
// I-17: Delegates to SessionManager.ResolveDirectionalPane so that the current pane
// resolution, window pane listing, and directional navigation all occur under a single
// lock acquisition, eliminating the TOCTOU race of three independent lock scopes.
func (r *CommandRouter) resolveDirectionalPane(req ipc.TmuxRequest) (*TmuxPane, error) {
	callerPaneID := ParseCallerPane(req.CallerPane)

	var direction DirectionalPaneDirection
	switch {
	case mustBool(req.Flags["-L"]), mustBool(req.Flags["-U"]):
		direction = DirPrev
	case mustBool(req.Flags["-R"]), mustBool(req.Flags["-D"]):
		direction = DirNext
	default:
		direction = DirNone
	}

	return r.sessions.ResolveDirectionalPane(callerPaneID, direction)
}
