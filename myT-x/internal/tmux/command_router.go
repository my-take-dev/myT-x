package tmux

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"strings"
	"sync"

	"myT-x/internal/ipc"
)

// DefaultTerminalCols is the default terminal width when no explicit size is provided.
const DefaultTerminalCols = 120

// DefaultTerminalRows is the default terminal height when no explicit size is provided.
const DefaultTerminalRows = 40

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
	ClaudeEnv     map[string]string // Claude Code env vars; protected by claudeEnvMu
	// OnSessionDestroyed is called after kill-session succeeds.
	// It runs outside of SessionManager locks.
	OnSessionDestroyed func(sessionName string)
	// OnSessionRenamed is called after rename-session succeeds.
	// It runs outside of SessionManager locks.
	OnSessionRenamed func(oldName, newName string)
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
	handlers    map[string]func(ipc.TmuxRequest) ipc.TmuxResponse
	// renamePane is a narrow test seam used to force non-fatal rename errors.
	renamePane func(paneID string, title string) (string, error)
	// attachTerminalFn is a test seam for attach/rollback paths.
	attachTerminalFn func(pane *TmuxPane, workDir string, env map[string]string, source *TmuxPane) error
	// getSessionForNewWindowFn is a narrow test seam for handleNewWindow's
	// snapshot-refetch rollback path.
	getSessionForNewWindowFn func(sessionName string) (*TmuxSession, bool)
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
	maps.Copy(copied, paneEnv)
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
// Callers MUST NOT mutate the returned map.
//
// INVARIANT (copy-on-write contract): This method returns a direct reference
// to the internal map, not a deep copy. Thread safety depends on the following
// invariant maintained by UpdatePaneEnv:
//   - UpdatePaneEnv always allocates a NEW map, copies entries into it, then
//     atomically swaps opts.PaneEnv under paneEnvMu. It NEVER performs in-place
//     writes to an existing map.
//   - Because the map reference is replaced atomically, any reference obtained
//     via paneEnvView remains immutable after the RLock is released.
//   - This contract has no compile-time enforcement. Any future writer that
//     mutates opts.PaneEnv in place (e.g., opts.PaneEnv[k] = v without
//     replacing the map) will introduce a data race.
//
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
	maps.Copy(copied, src)
	return copied
}

// UpdateClaudeEnv atomically replaces the Claude Code environment map.
// The caller's map is deep-copied; subsequent mutations to the caller's
// map do not affect the router's state.
func (r *CommandRouter) UpdateClaudeEnv(claudeEnv map[string]string) {
	var copied map[string]string
	if claudeEnv != nil {
		copied = make(map[string]string, len(claudeEnv))
		maps.Copy(copied, claudeEnv)
	}
	r.claudeEnvMu.Lock()
	r.opts.ClaudeEnv = copied
	r.claudeEnvMu.Unlock()
	slog.Debug("[DEBUG-ROUTER] ClaudeEnv updated", "count", len(copied))
}

// claudeEnvView returns a read-only map reference of current ClaudeEnv.
// Callers MUST NOT mutate the returned map.
//
// INVARIANT (copy-on-write contract): This method returns a direct reference
// to the internal map, not a deep copy. Thread safety depends on the following
// invariant maintained by UpdateClaudeEnv:
//   - UpdateClaudeEnv always allocates a NEW map, copies entries into it, then
//     atomically swaps opts.ClaudeEnv under claudeEnvMu. It NEVER performs
//     in-place writes to an existing map.
//   - Because the map reference is replaced atomically, any reference obtained
//     via claudeEnvView remains immutable after the RLock is released.
//   - This contract has no compile-time enforcement. Any future writer that
//     mutates opts.ClaudeEnv in place (e.g., opts.ClaudeEnv[k] = v without
//     replacing the map) will introduce a data race.
//
// See UpdateClaudeEnv for the writer side of this contract.
func (r *CommandRouter) claudeEnvView() map[string]string {
	r.claudeEnvMu.RLock()
	defer r.claudeEnvMu.RUnlock()
	return r.opts.ClaudeEnv
}

// ClaudeEnvSnapshot returns a deep copy of the current Claude Code env for testing.
func (r *CommandRouter) ClaudeEnvSnapshot() map[string]string {
	r.claudeEnvMu.RLock()
	src := r.opts.ClaudeEnv
	r.claudeEnvMu.RUnlock()
	if src == nil {
		return nil
	}
	cp := make(map[string]string, len(src))
	maps.Copy(cp, src)
	return cp
}

// resolveEnvForPaneCreation builds the environment variable map for a new pane.
// It branches between the new path (session-level UseClaudeEnv/UsePaneEnv flags)
// and the legacy path (buildPaneEnv with fill-only pane_env defaults).
//
// Nil defaults for the new path:
//   - UseClaudeEnv nil → false (no Claude env applied; conservative default)
//   - UsePaneEnv   nil → true  (fill-only applied; matches legacy behavior)
//
// Parameters:
//   - sessionSnap: deep-cloned snapshot (e.g. from GetSession) or nil for legacy fallback.
//     When non-nil, the internal GetSession lookup is skipped to avoid redundant deep clones.
//     Callers must NOT pass a live session pointer; always pass a clone or nil.
//   - sessionName: session to look up env flags from (used only when sessionSnap is nil).
//   - inheritedEnv: source pane env carried over from the originating pane.
//     When nil (e.g. new-window with no source pane), no inherited variables are merged;
//     the resulting env contains only config-level defaults, shimEnv, and tmux internals.
//   - shimEnv: env vars from shim -e flag or request env.
//   - sessionID, paneID: identifiers for tmux internal env vars.
func (r *CommandRouter) resolveEnvForPaneCreation(
	sessionSnap *TmuxSession,
	sessionName string,
	inheritedEnv, shimEnv map[string]string,
	sessionID, paneID int,
) map[string]string {
	if sessionSnap == nil {
		var ok bool
		sessionSnap, ok = r.sessions.GetSession(sessionName)
		if !ok {
			slog.Debug("[DEBUG-ENV] resolveEnvForPaneCreation: session not found, falling back to legacy path",
				"session", sessionName)
			sessionSnap = nil
		}
	}

	if sessionSnap != nil && (sessionSnap.UseClaudeEnv != nil || sessionSnap.UsePaneEnv != nil) {
		// New path: at least one flag was explicitly set.
		// Nil defaults:
		//   - UseClaudeEnv nil → false (no Claude env applied; conservative default)
		//   - UsePaneEnv   nil → true  (fill-only applied; matches legacy behavior)
		useClaudeEnv := sessionSnap.UseClaudeEnv != nil && *sessionSnap.UseClaudeEnv
		usePaneEnv := sessionSnap.UsePaneEnv == nil || *sessionSnap.UsePaneEnv
		return r.buildPaneEnvForSession(inheritedEnv, shimEnv, sessionID, paneID, useClaudeEnv, usePaneEnv)
	}

	// Legacy path: existing buildPaneEnv (pane_env always fills via fill-only)
	mergedReqEnv := copyEnvMap(inheritedEnv)
	maps.Copy(mergedReqEnv, shimEnv)
	return r.buildPaneEnv(mergedReqEnv, sessionID, paneID)
}

// buildPaneEnvForSession builds environment for additional panes, respecting
// session-level UseClaudeEnv and UsePaneEnv flags.
//
// Merge priority (lowest -> highest):
//  1. claude_env from config (fills base, when useClaudeEnv)
//  2. inheritedEnv (source pane env, includes claude_env if previously set)
//  3. pane_env from config (when usePaneEnv; overwrite if useClaudeEnv also true, fill-only otherwise)
//  4. shimEnv (shim's -e flag, highest custom priority)
//  5. tmux internal vars (always final)
//
// Snapshot consistency: claudeEnvMu and paneEnvMu are each acquired once under a
// single RLock, ensuring that all env reads within a single buildPaneEnvForSession
// call see a consistent view. This avoids redundant deep-clones inside
// resolveEnvForPaneCreation.
func (r *CommandRouter) buildPaneEnvForSession(
	inheritedEnv, shimEnv map[string]string,
	sessionID, paneID int,
	useClaudeEnv, usePaneEnv bool,
) map[string]string {
	// Snapshot env views once to avoid redundant RLock/RUnlock and ensure
	// consistency within a single buildPaneEnvForSession call.
	var claudeVars map[string]string
	var paneVars map[string]string
	if useClaudeEnv {
		claudeVars = r.claudeEnvView()
	}
	if usePaneEnv {
		paneVars = r.paneEnvView()
	}

	// Capacity hint: sum all contributors to minimize rehashing.
	// +6 accounts for shimEnv entries and tmux internal vars (Layer 5:
	// GO_TMUX, GO_TMUX_PANE, TMUX, TMUX_PANE, GO_TMUX_USER, headroom).
	capacity := len(inheritedEnv) + len(shimEnv) + 6
	if claudeVars != nil {
		capacity += len(claudeVars)
	}
	if paneVars != nil {
		capacity += len(paneVars)
	}
	env := make(map[string]string, capacity)

	// Layer 1: claude_env from config (fill base)
	// NOTE: blocked-key filtering is intentionally omitted here; claude_env is
	// admin-controlled config. Blocked system keys (PATH, SYSTEMROOT, etc.)
	// are enforced by Layers 2/4 (isBlockedEnvironmentKey) and downstream
	// mergeEnvironment → sanitizeCustomEnvironmentEntry. Layer 5
	// (addTmuxEnvironment) unconditionally overwrites tmux-internal keys only.
	if useClaudeEnv {
		maps.Copy(env, claudeVars)
	}

	// Layer 2: inherited env (overwrites claude_env)
	for k, v := range inheritedEnv {
		if isBlockedEnvironmentKey(k) {
			continue
		}
		env[k] = v
	}

	// Layer 3: pane_env from config
	// NOTE: blocked-key filtering is intentionally omitted here; pane_env is
	// admin-controlled config. Blocked system keys (PATH, SYSTEMROOT, etc.)
	// are enforced by Layers 2/4 (isBlockedEnvironmentKey) and downstream
	// mergeEnvironment → sanitizeCustomEnvironmentEntry. Layer 5
	// (addTmuxEnvironment) unconditionally overwrites tmux-internal keys only.
	if usePaneEnv {
		if useClaudeEnv {
			// When both are ON, pane_env overwrites (spec: "追加ペインが優先")
			maps.Copy(env, paneVars)
		} else {
			// Fill-only mode (backward compatible)
			mergePaneEnvDefaults(env, paneVars)
		}
	}

	// Layer 4: shim env (-e flag, highest custom priority)
	for k, v := range shimEnv {
		if isBlockedEnvironmentKey(k) {
			continue
		}
		env[k] = v
	}

	// Layer 5: tmux internal vars (always final)
	addTmuxEnvironment(env, r.opts.PipeName, r.opts.HostPID, sessionID, paneID, r.ShimAvailable())

	return env
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
	}
	router.renamePane = sessions.RenamePane
	router.attachTerminalFn = router.attachTerminal
	router.getSessionForNewWindowFn = sessions.GetSession
	router.handlers = map[string]func(ipc.TmuxRequest) ipc.TmuxResponse{
		"new-session":      router.handleNewSession,
		"has-session":      router.handleHasSession,
		"split-window":     router.handleSplitWindow,
		"send-keys":        router.handleSendKeys,
		"select-pane":      router.handleSelectPane,
		"list-sessions":    router.handleListSessions,
		"kill-session":     router.handleKillSession,
		"list-panes":       router.handleListPanes,
		"display-message":  router.handleDisplayMessage,
		"activate-window":  router.handleActivateWindow,
		"attach-session":   router.handleAttachSession,
		"kill-pane":        router.handleKillPane,
		"rename-session":   router.handleRenameSession,
		"resize-pane":      router.handleResizePane,
		"show-environment": router.handleShowEnvironment,
		"set-environment":  router.handleSetEnvironment,
		"list-windows":     router.handleListWindows,
		"rename-window":    router.handleRenameWindow,
		"new-window":       router.handleNewWindow,
		"kill-window":      router.handleKillWindow,
		"select-window":    router.handleSelectWindow,
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
	if _, err := term.Write(payload); err != nil {
		slog.Warn("["+debugTag+"] initial send-keys failed; continuing",
			"session", sessionName,
			"paneId", pane.IDString(),
			"error", err,
		)
	}
}

// buildPaneEnv builds the environment map for a new pane, merging request env,
// pane env defaults, and tmux-specific variables.
//
// TODO: Remove legacy buildPaneEnv when all sessions have explicit env flags.
func (r *CommandRouter) buildPaneEnv(reqEnv map[string]string, sessionID int, paneID int) map[string]string {
	env := make(map[string]string, len(reqEnv))
	for k, v := range reqEnv {
		if isBlockedEnvironmentKey(k) {
			continue
		}
		env[k] = v
	}
	mergePaneEnvDefaults(env, r.paneEnvView())
	addTmuxEnvironment(env, r.opts.PipeName, r.opts.HostPID, sessionID, paneID, r.ShimAvailable())
	return env
}

// buildPaneEnvSkipDefaults builds the environment map without merging pane_env
// config defaults. Used for operator-initiated initial session panes where
// pane_env settings (effort level, custom env vars) should not be applied.
func (r *CommandRouter) buildPaneEnvSkipDefaults(reqEnv map[string]string, sessionID int, paneID int) map[string]string {
	env := make(map[string]string, len(reqEnv))
	for k, v := range reqEnv {
		if isBlockedEnvironmentKey(k) {
			continue
		}
		env[k] = v
	}
	// NOTE: mergePaneEnvDefaults is intentionally skipped here.
	// Operator-initiated panes do not need agent-specific env vars.
	addTmuxEnvironment(env, r.opts.PipeName, r.opts.HostPID, sessionID, paneID, r.ShimAvailable())
	return env
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
