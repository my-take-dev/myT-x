package tmux

import (
	"fmt"
	"log/slog"
	"strings"

	"myT-x/internal/ipc"
)

// SplitWindowInternal is a typed fast-path for in-process GUI calls.
func (r *CommandRouter) SplitWindowInternal(targetPaneID string, horizontal bool) (string, error) {
	targetPaneID = strings.TrimSpace(targetPaneID)
	if targetPaneID == "" {
		return "", fmt.Errorf("missing required pane id")
	}

	target, err := r.sessions.ResolveTarget(targetPaneID, -1)
	if err != nil {
		return "", err
	}

	direction := SplitVertical
	if horizontal {
		direction = SplitHorizontal
	}
	newPane, err := r.splitWindowResolved(target, direction, "", nil, nil)
	if err != nil {
		return "", err
	}
	return newPane.IDString(), nil
}

// CreatePaneInEmptySessionInternal is a typed fast-path for recreating the
// first pane in an existing empty session.
func (r *CommandRouter) CreatePaneInEmptySessionInternal(sessionName string) (string, error) {
	sessionName = parseSessionName(sessionName)
	if sessionName == "" {
		return "", fmt.Errorf("session name is required")
	}

	sessionSnap, ok := r.sessions.GetSession(sessionName)
	if !ok {
		return "", fmt.Errorf("session not found: %s", sessionName)
	}
	workDir := sessionWorkDir(sessionSnap)

	_, newPane, err := r.sessions.CreatePaneInEmptySession(sessionName, DefaultTerminalCols, DefaultTerminalRows)
	if err != nil {
		return "", err
	}

	rollbackPane := func(stage string, originalErr error) error {
		if _, _, rollbackErr := r.sessions.KillPane(newPane.IDString()); rollbackErr != nil {
			slog.Warn("[WARN-PANE] failed to rollback pane after empty-session recreation error",
				"stage", stage,
				"paneId", newPane.IDString(),
				"originalErr", originalErr,
				"rollbackErr", rollbackErr,
			)
		}
		return originalErr
	}

	paneCtx, paneCtxErr := r.sessions.GetPaneContextSnapshot(newPane.ID)
	if paneCtxErr != nil {
		return "", rollbackPane("snapshot", paneCtxErr)
	}

	refreshedSessionSnap, refreshedOK := r.sessions.GetSession(sessionName)
	if !refreshedOK {
		return "", rollbackPane("session-refetch", fmt.Errorf("session disappeared during pane setup: %s", sessionName))
	}
	env := r.resolveEnvForPaneCreation(refreshedSessionSnap, sessionName, nil, nil, paneCtx.SessionID, newPane.ID)

	if attachErr := r.attachPaneTerminal(newPane, workDir, env, nil); attachErr != nil {
		return "", rollbackPane("attach-terminal", attachErr)
	}

	r.emitter.Emit("tmux:pane-created", map[string]any{
		"sessionName": paneCtx.SessionName,
		"paneId":      newPane.IDString(),
		"env":         env,
		"layout":      paneCtx.Layout,
	})
	r.emitter.Emit("tmux:layout-changed", map[string]any{
		"sessionName": paneCtx.SessionName,
		"layoutTree":  paneCtx.Layout,
	})

	return newPane.IDString(), nil
}

func (r *CommandRouter) splitWindowResolved(target *TmuxPane, direction SplitDirection, workDir string, extraEnv map[string]string, args []string) (*TmuxPane, error) {
	if target == nil || target.Window == nil || target.Window.Session == nil {
		return nil, fmt.Errorf("invalid split target")
	}

	targetPaneID := target.ID
	targetCtx, err := r.sessions.GetPaneContextSnapshot(targetPaneID)
	if err != nil {
		return nil, err
	}

	requestedWorkDir := workDir
	workDir = strings.TrimSpace(workDir)

	// Fallback: when workDir is not explicitly provided (GUI split path),
	// use the session's effective working directory.
	if workDir == "" {
		workDir = strings.TrimSpace(targetCtx.SessionWorkDir)
		if workDir != "" {
			slog.Debug("[DEBUG-SPLIT] splitWindowResolved: using session workdir fallback",
				"requestedWorkDir", requestedWorkDir,
				"sessionWorkDir", targetCtx.SessionWorkDir,
				"resolvedWorkDir", workDir,
			)
		} else {
			slog.Debug("[DEBUG-SPLIT] splitWindowResolved: workdir unresolved after fallback",
				"requestedWorkDir", requestedWorkDir,
				"sessionWorkDir", targetCtx.SessionWorkDir,
			)
		}
	}

	newPane, err := r.sessions.SplitPane(targetPaneID, direction)
	if err != nil {
		return nil, err
	}
	layoutSnapshot, layoutErr := r.sessions.paneLayoutSnapshot(newPane.ID)
	if layoutErr != nil {
		if _, _, rollbackErr := r.sessions.KillPane(newPane.IDString()); rollbackErr != nil {
			slog.Warn("[WARN-SPLIT] failed to rollback pane after layout snapshot error",
				"paneId", newPane.IDString(), "layoutErr", layoutErr, "rollbackErr", rollbackErr)
			return nil, layoutErr
		}
		return nil, layoutErr
	}

	slog.Debug("[DEBUG-SPLIT] splitWindowResolved: request args",
		"originalArgs", fmt.Sprintf("%v", args),
		"requestedWorkDir", requestedWorkDir,
		"resolvedWorkDir", workDir,
	)

	// Reuse a pre-fetched session snapshot to avoid a redundant deep clone
	// inside resolveEnvForPaneCreation (same pattern as handleNewWindow).
	// NOTE: snapshot failure is non-fatal; resolveEnvForPaneCreation falls back
	// to a direct GetSession call internally if sessionSnap is nil.
	sessionSnap, ok := r.sessions.GetSession(targetCtx.SessionName)
	if !ok {
		sessionSnap = nil
		slog.Warn("[WARN-ENV] splitWindowResolved: session not found for snapshot, falling back to legacy path",
			"session", targetCtx.SessionName)
	}
	env := r.resolveEnvForPaneCreation(sessionSnap, targetCtx.SessionName, targetCtx.Env, extraEnv, targetCtx.SessionID, newPane.ID)

	if attachErr := r.attachPaneTerminal(newPane, workDir, env, nil); attachErr != nil {
		if _, _, rollbackErr := r.sessions.KillPane(newPane.IDString()); rollbackErr != nil {
			slog.Warn("[WARN-SPLIT] failed to rollback pane after attachTerminal error",
				"paneId", newPane.IDString(), "attachErr", attachErr, "rollbackErr", rollbackErr)
			return nil, attachErr
		}
		return nil, attachErr
	}
	// send-keys bootstrap is best-effort: pane creation already succeeded, and
	// tmux-shim contract prefers forwarding over aborting on transform/write failures.
	r.bestEffortSendKeys(newPane, args, true, "DEBUG-SPLIT", targetCtx.SessionName)

	r.emitter.Emit("tmux:pane-created", map[string]any{
		"sessionName": targetCtx.SessionName,
		"paneId":      newPane.IDString(),
		"env":         env,
		"layout":      layoutSnapshot,
	})
	r.emitter.Emit("tmux:layout-changed", map[string]any{
		"sessionName": targetCtx.SessionName,
		"layoutTree":  layoutSnapshot,
	})

	return newPane, nil
}

func (r *CommandRouter) handleSplitWindow(req ipc.TmuxRequest) ipc.TmuxResponse {
	slog.Debug("[DEBUG-SPLIT] handleSplitWindow called",
		"target", mustString(req.Flags["-t"]),
		"workDir", mustString(req.Flags["-c"]),
		"args", fmt.Sprintf("%v", req.Args),
		"env", fmt.Sprintf("%v", req.Env),
		"callerPane", req.CallerPane,
	)

	target, err := r.resolveTargetFromRequest(req)
	if err != nil {
		return errResp(err)
	}
	direction := SplitVertical
	if mustBool(req.Flags["-h"]) {
		direction = SplitHorizontal
	}
	newPane, err := r.splitWindowResolved(target, direction, mustString(req.Flags["-c"]), req.Env, req.Args)
	if err != nil {
		return errResp(err)
	}

	// -d: keep focus on original pane (don't switch to new pane)
	if mustBool(req.Flags["-d"]) {
		if err := r.sessions.SetActivePane(target.ID); err != nil {
			slog.Debug("[DEBUG-SPLIT] failed to restore active pane", "error", err)
		}
	}

	// -P with -F: format output using tmux format variables.
	// NOTE (I-03 TOCTOU-safe pattern): Use expandFormatSafe instead of passing
	// the live newPane pointer to expandFormat. newPane.Window.Session is a live
	// pointer chain that can be concurrently mutated after lock release.
	// expandFormatSafe obtains a deep-cloned pane for safe traversal.
	if mustBool(req.Flags["-P"]) {
		format := mustString(req.Flags["-F"])
		if format == "" {
			format = "#{pane_id}"
		}
		return okResp(expandFormatSafe(format, newPane.ID, r.sessions) + "\n")
	}

	return okResp(fmt.Sprintf("%s\n", newPane.IDString()))
}

func hasSelectPaneDirectionalFlag(req ipc.TmuxRequest) bool {
	return mustBool(req.Flags["-L"]) ||
		mustBool(req.Flags["-R"]) ||
		mustBool(req.Flags["-U"]) ||
		mustBool(req.Flags["-D"])
}

func selectPaneTitle(req ipc.TmuxRequest) (title string, hasTitle bool) {
	rawTitle, hasTitle := req.Flags["-T"]
	if !hasTitle {
		return "", false
	}
	return strings.TrimSpace(mustString(rawTitle)), true
}

func (r *CommandRouter) applyPaneTitle(target *TmuxPane, fallbackSessionName string, title string) {
	if target == nil {
		slog.Debug("[DEBUG-SELECTPANE] skip pane title update: target pane is nil")
		return
	}
	renamePane := r.renamePane
	if renamePane == nil {
		renamePane = r.sessions.RenamePane
	}
	resolvedSessionName, err := renamePane(target.IDString(), title)
	if err != nil {
		slog.Debug("[DEBUG-SELECTPANE] failed to set pane title", "paneId", target.IDString(), "error", err)
		return
	}
	if strings.TrimSpace(resolvedSessionName) == "" {
		resolvedSessionName = fallbackSessionName
	}
	r.emitter.Emit("tmux:pane-renamed", map[string]any{
		"sessionName": resolvedSessionName,
		"paneId":      target.IDString(),
		"title":       title,
	})
}

func (r *CommandRouter) handleSelectPane(req ipc.TmuxRequest) ipc.TmuxResponse {
	targetSpecified := strings.TrimSpace(mustString(req.Flags["-t"])) != ""
	directionalSelection := hasSelectPaneDirectionalFlag(req)
	title, hasPaneTitle := selectPaneTitle(req)

	var target *TmuxPane
	var err error

	if targetSpecified {
		target, err = r.resolveTargetFromRequest(req)
		if err != nil {
			return errResp(err)
		}
	} else {
		target, err = r.resolveDirectionalPane(req)
		if err != nil {
			return errResp(err)
		}
	}

	if target == nil {
		return errResp(fmt.Errorf("invalid pane target"))
	}
	targetCtx, targetCtxErr := r.sessions.GetPaneContextSnapshot(target.ID)
	if targetCtxErr != nil {
		return errResp(targetCtxErr)
	}
	// tmux select-pane -T without -t/-U/-D/-L/-R updates the current pane title
	// without changing focus.
	if hasPaneTitle && !targetSpecified && !directionalSelection {
		r.applyPaneTitle(target, targetCtx.SessionName, title)
		return okResp("")
	}
	if err := r.sessions.SetActivePane(target.ID); err != nil {
		return errResp(err)
	}

	if hasPaneTitle {
		r.applyPaneTitle(target, targetCtx.SessionName, title)
	}

	r.emitter.Emit("tmux:pane-focused", map[string]any{
		"sessionName": targetCtx.SessionName,
		"paneId":      target.IDString(),
	})
	return okResp("")
}
