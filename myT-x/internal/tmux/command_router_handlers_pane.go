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
			slog.Warn("[DEBUG-SPLIT] failed to rollback pane after layout snapshot error",
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
			slog.Warn("[DEBUG-SPLIT] failed to rollback pane after attachTerminal error",
				"paneId", newPane.IDString(), "attachErr", attachErr, "rollbackErr", rollbackErr)
			return nil, attachErr
		}
		return nil, attachErr
	}
	if strings.TrimSpace(targetCtx.Title) != "" {
		if _, titleErr := r.sessions.RenamePane(newPane.IDString(), targetCtx.Title); titleErr != nil {
			slog.Debug("[DEBUG-SPLIT] failed to copy source pane title", "paneId", newPane.IDString(), "error", titleErr)
		}
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

func (r *CommandRouter) handleSendKeys(req ipc.TmuxRequest) ipc.TmuxResponse {
	slog.Debug("[DEBUG-SENDKEYS] handleSendKeys called",
		"target", mustString(req.Flags["-t"]),
		"callerPane", req.CallerPane,
		"argsCount", len(req.Args),
		"args", fmt.Sprintf("%v", req.Args),
	)

	target, err := r.resolveTargetFromRequest(req)
	if err != nil {
		return errResp(err)
	}
	// NOTE: target.Terminal is a live-pointer read after lock release. This is safe in
	// practice because Terminal is set once during pane creation (attachPaneTerminal)
	// and cleared only when the pane is killed. A concurrent kill between ResolveTarget
	// and Write would cause Write to fail gracefully rather than panic.
	if target.Terminal == nil {
		return errResp(fmt.Errorf("pane has no terminal: %s", target.IDString()))
	}
	payload := TranslateSendKeys(req.Args)

	slog.Debug("[DEBUG-SENDKEYS] writing to pane",
		"targetPane", target.IDString(),
		"payloadLen", len(payload),
		"payloadPreview", truncateBytes(payload, 200),
	)

	if len(payload) == 0 {
		slog.Debug("[DEBUG-SENDKEYS] empty payload after translation, skipping write",
			"targetPane", target.IDString(),
			"argsCount", len(req.Args),
		)
		return okResp("")
	}
	if _, err := target.Terminal.Write(payload); err != nil {
		return errResp(err)
	}
	return okResp("")
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

func (r *CommandRouter) emitLayoutChangedForSession(sessionName string, preferredWindowID int, debugTag string) {
	session, ok := r.sessions.GetSession(sessionName)
	if !ok {
		// INFO not Warn: session absence is expected in normal concurrent flows
		// (e.g., last pane killed removes session before layout event fires).
		message := "session not found, skipping layout event"
		if debugTag == "DEBUG-KILLPANE" {
			message = "session not found after kill, skipping layout event"
		}
		slog.Info("["+debugTag+"] "+message, "session", sessionName)
		return
	}

	survivingPane := paneForLayoutSnapshot(session, preferredWindowID)
	if survivingPane == nil {
		// INFO not Warn: no surviving pane is a normal outcome when the last
		// pane in a session is killed concurrently.
		slog.Info("["+debugTag+"] no surviving pane found, skipping layout event", "session", sessionName)
		return
	}
	// NOTE: GetSession returns a clone and cloned panes have Terminal=nil by design.
	// This path only consumes pane IDs for snapshot lookup.

	layoutSnapshot, layoutErr := r.sessions.paneLayoutSnapshot(survivingPane.ID)
	if layoutErr != nil {
		// INFO not Warn: snapshot failure after concurrent kill is expected.
		slog.Info("["+debugTag+"] failed to get layout snapshot", "error", layoutErr)
		return
	}

	r.emitter.Emit("tmux:layout-changed", map[string]any{
		"sessionName": sessionName,
		"layoutTree":  layoutSnapshot,
	})
}

func paneForLayoutSnapshot(session *TmuxSession, preferredWindowID int) *TmuxPane {
	if session == nil {
		return nil
	}
	if preferredWindowID >= 0 {
		for _, window := range session.Windows {
			if window == nil || window.ID != preferredWindowID || len(window.Panes) == 0 {
				continue
			}
			if window.ActivePN >= 0 && window.ActivePN < len(window.Panes) {
				if pane := window.Panes[window.ActivePN]; pane != nil {
					return pane
				}
			}
			if pane := window.Panes[0]; pane != nil {
				return pane
			}
		}
	}
	return firstPaneInSession(session)
}

func (r *CommandRouter) handleKillPane(req ipc.TmuxRequest) ipc.TmuxResponse {
	target, err := r.resolveTargetFromRequest(req)
	if err != nil {
		return errResp(err)
	}

	// KillPane closes the terminal internally (via killPaneLocked -> closeTargets).
	// Do NOT capture terminal here to avoid double-close (C-01).
	paneID := target.IDString()

	// NOTE: Snapshot session/window context before KillPane so that layout events
	// can reference the correct session name and window ID. Reading target.Window
	// or target.Window.Session after ResolveTarget is a live-pointer read without
	// lock protection (I-07). GetPaneContextSnapshot acquires RLock internally.
	targetCtx, ctxErr := r.sessions.GetPaneContextSnapshot(target.ID)
	sessionName := targetCtx.SessionName
	preferredWindowID := targetCtx.WindowID
	if ctxErr != nil {
		// Snapshot failure is non-fatal: KillPane returns the session name as fallback.
		sessionName = ""
		preferredWindowID = -1
	}

	sName, removedSession, killErr := r.sessions.KillPane(paneID)
	if killErr != nil {
		return errResp(killErr)
	}
	if sessionName == "" {
		sessionName = sName
	}

	if removedSession {
		r.emitter.Emit("tmux:session-destroyed", map[string]any{
			"name": sessionName,
		})
	} else {
		r.emitLayoutChangedForSession(sessionName, preferredWindowID, "DEBUG-KILLPANE")
	}

	return okResp("")
}

func (r *CommandRouter) handleResizePane(req ipc.TmuxRequest) ipc.TmuxResponse {
	// I-01: Log warning when direction flags are present but not yet implemented.
	// The shim parses -U/-D/-L/-R/-Z (see spec.go resize-pane) and forwards them,
	// but this handler only supports explicit -x/-y sizing for now.
	if hasResizePaneDirectionFlag(req) {
		slog.Warn("[tmux-compat] resize-pane direction flags not yet implemented",
			"flagU", mustBool(req.Flags["-U"]),
			"flagD", mustBool(req.Flags["-D"]),
			"flagL", mustBool(req.Flags["-L"]),
			"flagR", mustBool(req.Flags["-R"]),
			"flagZ", mustBool(req.Flags["-Z"]),
		)
	}

	target, err := r.resolveTargetFromRequest(req)
	if err != nil {
		return errResp(err)
	}

	// NOTE (I-02 TOCTOU-safe pattern): Use GetPaneContextSnapshot to read
	// fallback Width/Height under RLock instead of dereferencing the live
	// pointer after lock release. The snapshot values serve only as fallback
	// defaults when the caller omits -x/-y flags; ResizePane re-validates
	// dimensions under its own lock, so a stale fallback is harmless.
	paneID := target.ID
	preCtx, preCtxErr := r.sessions.GetPaneContextSnapshot(paneID)
	fallbackCols := DefaultTerminalCols
	fallbackRows := DefaultTerminalRows
	if preCtxErr == nil {
		fallbackCols = preCtx.PaneWidth
		fallbackRows = preCtx.PaneHeight
	}

	cols := mustInt(req.Flags["-x"], fallbackCols)
	rows := mustInt(req.Flags["-y"], fallbackRows)

	if resizeErr := r.sessions.ResizePane(target.IDString(), cols, rows); resizeErr != nil {
		return errResp(resizeErr)
	}

	// Re-snapshot after resize to get the updated layout for the event.
	// Fall back to pre-resize snapshot for session name / window ID if the
	// post-resize snapshot fails (pane killed concurrently).
	postCtx, postCtxErr := r.sessions.GetPaneContextSnapshot(paneID)
	if postCtxErr != nil {
		if preCtxErr != nil {
			// Both snapshots failed — pane was killed. Layout event is
			// best-effort, so silently skip rather than fail the command.
			return okResp("")
		}
		postCtx = preCtx
	}
	r.emitLayoutChangedForSession(postCtx.SessionName, postCtx.WindowID, "DEBUG-RESIZEPANE")

	return okResp("")
}

// hasResizePaneDirectionFlag returns true when any directional resize flag is set.
func hasResizePaneDirectionFlag(req ipc.TmuxRequest) bool {
	return mustBool(req.Flags["-U"]) ||
		mustBool(req.Flags["-D"]) ||
		mustBool(req.Flags["-L"]) ||
		mustBool(req.Flags["-R"]) ||
		mustBool(req.Flags["-Z"])
}

func (r *CommandRouter) handleListPanes(req ipc.TmuxRequest) ipc.TmuxResponse {
	target := mustString(req.Flags["-t"])
	allInSession := mustBool(req.Flags["-s"])
	format := mustString(req.Flags["-F"])
	callerPaneID := ParseCallerPane(req.CallerPane)

	panes, err := r.sessions.ListPanesByWindowTarget(target, callerPaneID, allInSession)
	if err != nil {
		return errResp(err)
	}
	// I-4: panes is []TmuxPane (value copies) to prevent data-race on internal
	// pointers. The slice is already in correct order (window-by-window, then
	// pane-by-pane within each window) from the collection loop in
	// ListPanesByWindowTarget. No re-sorting is needed; value-copied panes
	// have Window=nil, making the original multi-level sort impossible.

	lines := make([]string, 0, len(panes))
	for i := range panes {
		lines = append(lines, formatPaneLine(&panes[i], format))
	}
	return okResp(joinLines(lines))
}
