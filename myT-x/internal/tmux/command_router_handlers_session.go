package tmux

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"myT-x/internal/ipc"
)

func (r *CommandRouter) handleNewSession(req ipc.TmuxRequest) ipc.TmuxResponse {
	sessionName := mustString(req.Flags["-s"])
	windowName := mustString(req.Flags["-n"])
	width := mustInt(req.Flags["-x"], DefaultTerminalCols)
	height := mustInt(req.Flags["-y"], DefaultTerminalRows)
	workDir := mustString(req.Flags["-c"])

	slog.Debug("[DEBUG-SESSION] handleNewSession called",
		"sessionName", sessionName,
		"workDir", workDir,
		"args", fmt.Sprintf("%v", req.Args),
		"env", fmt.Sprintf("%v", req.Env),
		"callerPane", req.CallerPane,
	)

	session, pane, err := r.sessions.CreateSession(sessionName, windowName, width, height)
	if err != nil {
		return errResp(err)
	}
	rollbackSession := func(stage string, originalErr error) ipc.TmuxResponse {
		if _, rmErr := r.sessions.RemoveSession(session.Name); rmErr != nil {
			slog.Warn("[WARN-SESSION] failed to remove session during rollback",
				"session", session.Name, "stage", stage, "originalErr", originalErr, "removeErr", rmErr)
			return errResp(originalErr)
		}
		return errResp(originalErr)
	}

	paneCtx, paneCtxErr := r.sessions.GetPaneContextSnapshot(pane.ID)
	if paneCtxErr != nil {
		return rollbackSession("snapshot", paneCtxErr)
	}

	if _, ok := req.Env["CLAUDE_CODE_AGENT_TYPE"]; ok {
		if setErr := r.sessions.SetAgentTeam(session.Name, true); setErr != nil {
			return rollbackSession("set-agent-team", setErr)
		}
	}

	// The initial pane of a new session always skips pane_env defaults.
	// pane_env settings (effort level, custom env vars) are intended for
	// additional panes only (split-window, new-window).
	// NOTE: new-window now creates a child session, but pane_env still applies
	// to the child session's initial pane via resolveEnvForPaneCreation.
	env := r.buildPaneEnvSkipDefaults(req.Env, paneCtx.SessionID, pane.ID)

	if err := r.attachPaneTerminal(pane, workDir, env, nil); err != nil {
		return rollbackSession("attach-terminal", err)
	}

	// send-keys bootstrap is best-effort: the session is already created and
	// we keep shim compatibility by not failing the command after creation.
	r.bestEffortSendKeys(pane, req.Args, true, "DEBUG-SESSION", paneCtx.SessionName)

	// I-16: Emit session-created regardless of -d flag.
	// The -d flag controls focus (detach), not whether the session was created.
	emitCtx, emitCtxErr := r.sessions.GetPaneContextSnapshot(pane.ID)
	if emitCtxErr != nil {
		slog.Debug("[DEBUG-SESSION] failed to refresh pane context for session-created event",
			"session", paneCtx.SessionName, "error", emitCtxErr)
		emitCtx = paneCtx
	}
	r.emitter.Emit("tmux:session-created", map[string]any{
		"name":          emitCtx.SessionName,
		"id":            emitCtx.SessionID,
		"initialPane":   pane.IDString(),
		"initialLayout": emitCtx.Layout,
	})

	// -P with -F: format output using tmux format variables.
	// NOTE (I-03 TOCTOU-safe pattern): Use expandFormatSafe instead of passing
	// the live pane pointer to expandFormat. pane.Window.Session is a live
	// pointer chain that can be concurrently mutated after lock release.
	// expandFormatSafe obtains a deep-cloned pane for safe traversal.
	if mustBool(req.Flags["-P"]) {
		format := mustString(req.Flags["-F"])
		if format == "" {
			format = "#{session_name}"
		}
		return okResp(expandFormatSafe(format, pane.ID, r.sessions) + "\n")
	}

	return okResp(fmt.Sprintf("%s\n", paneCtx.SessionName))
}

func (r *CommandRouter) handleListSessions(req ipc.TmuxRequest) ipc.TmuxResponse {
	format := mustString(req.Flags["-F"])
	sessions := r.sessions.ListSessions()
	lines := make([]string, 0, len(sessions))
	for _, session := range sessions {
		lines = append(lines, formatSessionLine(session, format))
	}
	return okResp(joinLines(lines))
}

func (r *CommandRouter) handleKillSession(req ipc.TmuxRequest) ipc.TmuxResponse {
	target := strings.TrimSpace(mustString(req.Flags["-t"]))
	if target == "" {
		return errResp(fmt.Errorf("missing required flag: -t"))
	}
	sessionName := parseSessionName(target)
	session, err := r.sessions.RemoveSession(sessionName)
	if err != nil {
		return errResp(err)
	}
	r.emitter.Emit("tmux:session-destroyed", map[string]any{
		"name": session.Name,
	})
	if r.opts.OnSessionDestroyed != nil {
		r.opts.OnSessionDestroyed(session.Name)
	}
	return okResp("")
}

func (r *CommandRouter) handleHasSession(req ipc.TmuxRequest) ipc.TmuxResponse {
	target := strings.TrimSpace(mustString(req.Flags["-t"]))
	if target == "" {
		return errResp(fmt.Errorf("missing required flag: -t"))
	}
	if r.sessions.HasSession(target) {
		return okResp("")
	}
	return ipc.TmuxResponse{ExitCode: 1}
}

func (r *CommandRouter) handleRenameSession(req ipc.TmuxRequest) ipc.TmuxResponse {
	target := strings.TrimSpace(mustString(req.Flags["-t"]))
	if len(req.Args) == 0 || strings.TrimSpace(req.Args[0]) == "" {
		return errResp(fmt.Errorf("rename-session requires new-name argument"))
	}
	newName := strings.TrimSpace(req.Args[0])

	oldName := target
	if oldName == "" {
		return errResp(fmt.Errorf("rename-session requires -t"))
	}
	oldName = parseSessionName(oldName)

	if err := r.sessions.RenameSession(oldName, newName); err != nil {
		return errResp(err)
	}

	r.emitter.Emit("tmux:session-renamed", map[string]any{
		"oldName": oldName,
		"newName": newName,
	})
	if r.opts.OnSessionRenamed != nil {
		r.opts.OnSessionRenamed(oldName, newName)
	}
	return okResp("")
}

func (r *CommandRouter) handleShowEnvironment(req ipc.TmuxRequest) ipc.TmuxResponse {
	target := strings.TrimSpace(mustString(req.Flags["-t"]))

	// -g (global) returns empty since myT-x has no global env concept
	if mustBool(req.Flags["-g"]) || target == "" {
		if mustBool(req.Flags["-g"]) {
			slog.Debug("[DEBUG-SESSION] show-environment: -g flag set, returning empty (no global env concept)")
		}
		return okResp("")
	}

	sessionName := parseSessionName(target)
	env, err := r.sessions.GetSessionEnv(sessionName)
	if err != nil {
		return errResp(err)
	}

	// If a variable name is specified, show only that variable
	if len(req.Args) > 0 && strings.TrimSpace(req.Args[0]) != "" {
		varName := strings.TrimSpace(req.Args[0])
		if val, ok := env[varName]; ok {
			return okResp(fmt.Sprintf("%s=%s\n", varName, val))
		}
		return ipc.TmuxResponse{ExitCode: 1, Stderr: fmt.Sprintf("unknown variable: %s\n", varName)}
	}

	// Sort and output all variables
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%s=%s", k, env[k]))
	}
	return okResp(joinLines(lines))
}

func (r *CommandRouter) handleSetEnvironment(req ipc.TmuxRequest) ipc.TmuxResponse {
	target := strings.TrimSpace(mustString(req.Flags["-t"]))

	// -g (global) is accepted but no-op for myT-x
	if mustBool(req.Flags["-g"]) {
		return okResp("")
	}

	if target == "" {
		return errResp(fmt.Errorf("set-environment requires -t"))
	}
	sessionName := parseSessionName(target)

	if len(req.Args) == 0 || strings.TrimSpace(req.Args[0]) == "" {
		return errResp(fmt.Errorf("set-environment requires variable name"))
	}
	varName := strings.TrimSpace(req.Args[0])

	if mustBool(req.Flags["-u"]) {
		if err := r.sessions.UnsetSessionEnv(sessionName, varName); err != nil {
			return errResp(err)
		}
		return okResp("")
	}

	if len(req.Args) < 2 {
		return errResp(fmt.Errorf("set-environment requires variable value"))
	}
	varValue := req.Args[1]

	if err := r.sessions.SetSessionEnv(sessionName, varName, varValue); err != nil {
		return errResp(err)
	}
	return okResp("")
}

// handleAttachSession activates the app window for the target session.
// Unlike real tmux, myT-x has no client connection concept. This handler
// simply emits an "app:activate-window" event to bring the host window
// to the foreground and returns success without producing stdout output.
func (r *CommandRouter) handleAttachSession(req ipc.TmuxRequest) ipc.TmuxResponse {
	target := strings.TrimSpace(mustString(req.Flags["-t"]))
	if target == "" {
		return errResp(fmt.Errorf("missing required flag: -t"))
	}
	resolvedSession := parseSessionName(target)
	if _, ok := r.sessions.GetSession(resolvedSession); !ok {
		return errResp(fmt.Errorf("session not found: %s", resolvedSession))
	}
	slog.Debug("[DEBUG-SESSION] attach-session command received", "target", target, "resolvedSession", resolvedSession)
	r.emitter.Emit("app:activate-window", nil)
	// tmux attach-session does not produce stdout on success.
	// NOTE: activate-window is an internal IPC command and intentionally returns "ok\n".
	return okResp("")
}
