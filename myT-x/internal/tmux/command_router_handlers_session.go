package tmux

import (
	"fmt"
	"log/slog"
	"strings"

	"myT-x/internal/ipc"
)

func (r *CommandRouter) handleNewSession(req ipc.TmuxRequest) ipc.TmuxResponse {
	sessionName := mustString(req.Flags["-s"])
	windowName := mustString(req.Flags["-n"])
	width := mustInt(req.Flags["-x"], 120)
	height := mustInt(req.Flags["-y"], 40)
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
			slog.Warn("[DEBUG-SESSION] failed to remove session during rollback",
				"session", session.Name, "stage", stage, "originalErr", originalErr, "removeErr", rmErr)
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

	env := copyEnvMap(req.Env)
	mergePaneEnvDefaults(env, r.getPaneEnv())
	addTmuxEnvironment(env, r.opts.PipeName, r.opts.HostPID, paneCtx.SessionID, pane.ID, r.ShimAvailable())

	if err := r.attachTerminal(pane, workDir, env, nil); err != nil {
		return rollbackSession("attach-terminal", err)
	}

	if len(req.Args) > 0 {
		// send-keys bootstrap is best-effort: the session is already created and
		// we keep shim compatibility by not failing the command after creation.
		payload := TranslateSendKeys(append(req.Args, "Enter"))
		if _, err := pane.Terminal.Write(payload); err != nil {
			slog.Warn("[DEBUG-SESSION] initial send-keys failed; continuing with created session",
				"session", paneCtx.SessionName,
				"paneId", pane.IDString(),
				"error", err,
			)
		}
	}

	if !mustBool(req.Flags["-d"]) {
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
	}

	// -P with -F: format output using tmux format variables
	if mustBool(req.Flags["-P"]) {
		format := mustString(req.Flags["-F"])
		if format == "" {
			format = "#{session_name}"
		}
		return okResp(expandFormat(format, pane) + "\n")
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
	session, err := r.sessions.RemoveSession(target)
	if err != nil {
		return errResp(err)
	}
	r.emitter.Emit("tmux:session-destroyed", map[string]any{
		"name": session.Name,
	})
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
