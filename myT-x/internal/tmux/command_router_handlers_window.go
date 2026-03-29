package tmux

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"myT-x/internal/ipc"
)

// handleActivateWindow signals the host application to bring its window
// to the foreground. Used by the second instance to activate the first
// instance's window before exiting.
func (r *CommandRouter) handleActivateWindow(_ ipc.TmuxRequest) ipc.TmuxResponse {
	slog.Debug("[DEBUG-IPC] activate-window command received")
	r.emitter.Emit("app:activate-window", nil)
	// Internal IPC callers expect "ok\n". attach-session intentionally keeps stdout empty.
	return ipc.TmuxResponse{ExitCode: 0, Stdout: "ok\n"}
}

// resolveWindowFromRequest resolves -t to a session name and window index (0-based position in the session's Windows slice).
// Target formats: "%paneID", "session" (active window), "session:windowIdx", "session:@windowID".
// Returns:
//   - sessionName: the resolved session name (always non-empty on success).
//   - windowIdx: the 0-based index of the window within session.Windows.
//   - err: non-nil when the target cannot be resolved.
func (r *CommandRouter) resolveWindowFromRequest(req ipc.TmuxRequest) (sessionName string, windowIdx int, err error) {
	sessionName, windowID, err := r.resolveWindowIDFromRequest(req)
	if err != nil {
		return "", 0, err
	}
	session, ok := r.sessions.GetSession(sessionName)
	if !ok {
		return "", 0, fmt.Errorf("session not found: %s", sessionName)
	}
	for i, window := range session.Windows {
		if window != nil && window.ID == windowID {
			return sessionName, i, nil
		}
	}
	return "", 0, fmt.Errorf("window not found in session: %s", sessionName)
}

// resolveWindowIDFromRequest resolves -t to a stable session name + window ID pair.
// Unlike resolveWindowFromRequest, the returned windowID is the window's stable ID
// (TmuxWindow.ID), not its positional index. This is safe across concurrent mutations
// that may reorder or remove windows.
// Returns:
//   - sessionName: the resolved session name (always non-empty on success).
//   - windowID: the stable TmuxWindow.ID (not an index).
//   - err: non-nil when the target cannot be resolved.
func (r *CommandRouter) resolveWindowIDFromRequest(req ipc.TmuxRequest) (sessionName string, windowID int, err error) {
	target := strings.TrimSpace(mustString(req.Flags["-t"]))
	if target == "" {
		return "", 0, fmt.Errorf("missing required flag: -t")
	}

	// Stable window ID target used by App window APIs to avoid index-based TOCTOU.
	if parsedSessionName, parsedWindowID, matched, parseErr := parseStableWindowIDTarget(target); matched {
		if parseErr != nil {
			return "", 0, parseErr
		}
		session, ok := r.sessions.GetSession(parsedSessionName)
		if !ok {
			return "", 0, fmt.Errorf("session not found: %s", parsedSessionName)
		}
		for _, window := range session.Windows {
			if window != nil && window.ID == parsedWindowID {
				return parsedSessionName, parsedWindowID, nil
			}
		}
		return "", 0, fmt.Errorf("window not found in session: %s", parsedSessionName)
	}

	// Resolve via pane to get the window context.
	pane, resolveErr := r.resolveTargetFromRequest(req)
	if resolveErr != nil {
		return "", 0, resolveErr
	}
	// NOTE: Use GetPaneContextSnapshot to read session name and window ID safely
	// instead of traversing the live pointer chain pane.Window.Session after lock release.
	paneCtx, ctxErr := r.sessions.GetPaneContextSnapshot(pane.ID)
	if ctxErr != nil {
		return "", 0, fmt.Errorf("cannot resolve window from target: %s", target)
	}
	return paneCtx.SessionName, paneCtx.WindowID, nil
}

func parseStableWindowIDTarget(target string) (sessionName string, windowID int, matched bool, err error) {
	target = strings.TrimSpace(target)
	sessionPart, suffix, hasColon := strings.Cut(target, ":")
	if !hasColon {
		return "", 0, false, nil
	}
	suffix = strings.TrimSpace(suffix)
	if !strings.HasPrefix(suffix, "@") {
		return "", 0, false, nil
	}
	sessionName = strings.TrimSpace(sessionPart)
	if sessionName == "" {
		return "", 0, true, fmt.Errorf("session name is required in target: %s", target)
	}
	windowIDText := strings.TrimSpace(strings.TrimPrefix(suffix, "@"))
	if windowIDText == "" {
		return "", 0, true, fmt.Errorf("window id is required in target: %s", target)
	}
	parsed, parseErr := strconv.Atoi(windowIDText)
	if parseErr != nil || parsed < 0 {
		return "", 0, true, fmt.Errorf("invalid window id in target: %s", target)
	}
	return sessionName, parsed, true, nil
}

func (r *CommandRouter) handleListWindows(req ipc.TmuxRequest) ipc.TmuxResponse {
	format := mustString(req.Flags["-F"])
	filter := mustString(req.Flags["-f"])
	allSessions := mustBool(req.Flags["-a"])
	target := strings.TrimSpace(mustString(req.Flags["-t"]))

	var sessions []*TmuxSession

	if allSessions {
		sessions = r.sessions.ListSessions()
	} else if target != "" {
		sessionName := parseSessionName(target)
		session, ok := r.sessions.GetSession(sessionName)
		if !ok {
			return errResp(fmt.Errorf("session not found: %s", sessionName))
		}
		sessions = []*TmuxSession{session}
	} else {
		// tmux-compatible default: no -t means current session.
		currentPane, resolveErr := r.sessions.ResolveTarget("", ParseCallerPane(req.CallerPane))
		if resolveErr != nil {
			return errResp(resolveErr)
		}
		// Use GetPaneContextSnapshot to safely read session name outside the lock.
		paneCtx, paneCtxErr := r.sessions.GetPaneContextSnapshot(currentPane.ID)
		if paneCtxErr != nil {
			return errResp(paneCtxErr)
		}
		sessionName := paneCtx.SessionName
		session, ok := r.sessions.GetSession(sessionName)
		if !ok {
			return errResp(fmt.Errorf("session not found: %s", sessionName))
		}
		sessions = []*TmuxSession{session}
	}

	lines := make([]string, 0)
	for _, session := range sessions {
		for _, window := range session.Windows {
			if window == nil {
				continue
			}
			// For filter evaluation, use the active pane of the window as context.
			// ListSessions/GetSession return deep clones with intact back-references.
			if filter != "" {
				// Use the same pane selection as formatWindowLine: active pane first,
				// then first non-nil pane as fallback.
				var filterPane *TmuxPane
				if window.ActivePN >= 0 && window.ActivePN < len(window.Panes) {
					filterPane = window.Panes[window.ActivePN]
				}
				if filterPane == nil {
					for _, p := range window.Panes {
						if p != nil {
							filterPane = p
							break
						}
					}
				}
				// If no non-nil pane exists, filter variables cannot be expanded.
				// Include the window (pass-through) rather than silently excluding it.
				if filterPane == nil {
					slog.Debug("[DEBUG-LISTWINDOWS] no non-nil pane in window, skipping filter evaluation",
						"windowID", window.ID, "windowName", window.Name)
					lines = append(lines, formatWindowLine(window, format))
					continue
				}
				if !evaluateFilter(filter, filterPane) {
					continue
				}
			}
			lines = append(lines, formatWindowLine(window, format))
		}
	}
	return okResp(joinLines(lines))
}

func (r *CommandRouter) handleRenameWindow(req ipc.TmuxRequest) ipc.TmuxResponse {
	if len(req.Args) == 0 || strings.TrimSpace(req.Args[0]) == "" {
		return errResp(fmt.Errorf("rename-window requires new-name argument"))
	}
	newName := strings.TrimSpace(req.Args[0])

	sessionName, windowID, err := r.resolveWindowIDFromRequest(req)
	if err != nil {
		return errResp(err)
	}

	windowIdx, renameErr := r.sessions.RenameWindowByID(sessionName, windowID, newName)
	if renameErr != nil {
		return errResp(renameErr)
	}

	r.emitter.Emit("tmux:window-renamed", map[string]any{
		"sessionName": sessionName,
		"windowIndex": windowIdx,
		"windowName":  newName,
	})
	return okResp("")
}
