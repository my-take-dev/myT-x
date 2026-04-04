// command_router_handlers_pane_lifecycle.go — Pane lifecycle handlers: kill, resize, layout events.
package tmux

import (
	"log/slog"

	"myT-x/internal/ipc"
)

func (r *CommandRouter) emitLayoutChangedForSession(sessionName string, preferredWindowID int, debugTag string) {
	session, ok := r.sessions.GetSession(sessionName)
	if !ok {
		// INFO not Warn: session absence is expected in normal concurrent flows
		// (e.g., explicit session removal before a delayed layout event fires).
		message := "session not found, skipping layout event"
		if debugTag == "DEBUG-KILLPANE" || debugTag == "DEBUG-KILLWINDOW" {
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

	sName, sessionEmptied, killErr := r.sessions.KillPane(paneID)
	if killErr != nil {
		return errResp(killErr)
	}
	if sessionName == "" {
		sessionName = sName
	}

	if sessionEmptied {
		r.emitter.Emit("tmux:session-emptied", map[string]any{
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

	cols := resolveDimension(req.Flags["-x"], fallbackCols, fallbackCols)
	rows := resolveDimension(req.Flags["-y"], fallbackRows, fallbackRows)

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
			// best-effort, so skip rather than fail the command.
			slog.Debug("[DEBUG-RESIZEPANE] both pre/post snapshots failed, skipping layout event",
				"paneId", paneID, "preErr", preCtxErr, "postErr", postCtxErr)
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
