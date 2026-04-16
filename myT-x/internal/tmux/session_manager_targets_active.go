// session_manager_targets_active.go — Active element resolution: session/window/pane target lookup and auto-repair.
package tmux

import (
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
)

// ResolveSessionTarget resolves a session from a target-like string.
//
// S-3: Delegates to resolveSessionTargetLocked to eliminate logic duplication
// between the public API and internal callers.
func (m *SessionManager) ResolveSessionTarget(target string) (*TmuxSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, err := m.resolveSessionTargetLocked(target)
	if err != nil {
		return nil, err
	}
	return cloneSessionForRead(session), nil
}

func (m *SessionManager) defaultPaneLocked() (*TmuxPane, error) {
	if len(m.sessions) == 0 {
		return nil, errors.New("no sessions")
	}
	sessions := make([]*TmuxSession, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].ID < sessions[j].ID })
	return m.activePaneInSessionLocked(sessions[0])
}

// defaultPaneRLocked is the read-only counterpart of defaultPaneLocked.
// Returns needsRepair=true when the resolved session has a stale ActiveWindowID.
func (m *SessionManager) defaultPaneRLocked() (*TmuxPane, bool, error) {
	if len(m.sessions) == 0 {
		return nil, false, errors.New("no sessions")
	}
	sessions := make([]*TmuxSession, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].ID < sessions[j].ID })
	return m.activePaneInSessionRLocked(sessions[0])
}

// resolveSessionTargetLocked resolves a session from target. Read-only operation.
//
// Naming convention note: This method uses the "Locked" suffix (not "RLocked")
// because it is called under both mu.RLock (ResolveSessionTarget) and mu.Lock
// (ListPanesByWindowTarget). The "Locked" suffix in this codebase means
// "caller must hold mu in some mode"; "RLocked" specifically means "RLock only,
// returns needsRepair for write-lock upgrade". Since this function never signals
// needsRepair, the generic "Locked" suffix is appropriate.
func (m *SessionManager) resolveSessionTargetLocked(target string) (*TmuxSession, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, errors.New("session target required")
	}
	if strings.HasPrefix(target, "%") {
		id, err := parsePaneID(target)
		if err != nil {
			return nil, err
		}
		pane, ok := m.panes[id]
		if !ok || pane == nil || pane.Window == nil || pane.Window.Session == nil {
			return nil, fmt.Errorf("pane not found: %s", target)
		}
		return pane.Window.Session, nil
	}
	if windowID, isWindowTarget, err := parseWindowIDTarget(target); isWindowTarget {
		if err != nil {
			return nil, err
		}
		_, ownerSession := m.findWindowByIDGlobalLocked(windowID)
		if ownerSession == nil {
			return nil, fmt.Errorf("window id not found: %d", windowID)
		}
		return ownerSession, nil
	}
	sessionName, _, _ := strings.Cut(target, ":")
	session, err := m.resolveSessionTokenLocked(sessionName)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, fmt.Errorf("session not found: %s", sessionName)
	}
	return session, nil
}

func (m *SessionManager) resolveSessionTokenLocked(token string) (*TmuxSession, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, nil
	}
	if sessionID, isSessionTarget, err := parseSessionIDTarget(token); isSessionTarget {
		if err != nil {
			return nil, err
		}
		session := m.findSessionByIDLocked(sessionID)
		if session == nil {
			return nil, fmt.Errorf("session id not found: %d", sessionID)
		}
		return session, nil
	}
	return m.sessions[token], nil
}

func (m *SessionManager) findSessionByIDLocked(sessionID int) *TmuxSession {
	for _, session := range m.sessions {
		if session == nil {
			continue
		}
		if session.ID == sessionID {
			return session
		}
	}
	return nil
}

func (m *SessionManager) activePaneInSessionLocked(session *TmuxSession) (*TmuxPane, error) {
	activeWindow := m.activeWindowInSessionLocked(session)
	if activeWindow == nil {
		return nil, errors.New("session has no windows")
	}
	return activePaneInWindow(activeWindow)
}

// activePaneInSessionRLocked resolves the active pane under RLock.
// Returns needsRepair=true when ActiveWindowID is stale and requires a write
// Lock to auto-repair. The pane returned may still be valid (the fallback
// window's active pane), but the caller should re-resolve under Lock to
// persist the repair.
func (m *SessionManager) activePaneInSessionRLocked(session *TmuxSession) (*TmuxPane, bool, error) {
	if session == nil || len(session.Windows) == 0 {
		return nil, false, errors.New("session has no windows")
	}
	activeWindow, fallback := findWindowByID(session.Windows, session.ActiveWindowID)
	if activeWindow != nil {
		pane, err := activePaneInWindow(activeWindow)
		return pane, false, err
	}
	// ActiveWindowID is stale. Return the fallback pane but signal the caller
	// that a write Lock is needed to persist the repair.
	if fallback == nil {
		return nil, false, errors.New("session has no windows")
	}
	pane, err := activePaneInWindow(fallback)
	if err != nil {
		return nil, false, err
	}
	return pane, true, nil
}

func findWindowByID(windows []*TmuxWindow, activeWindowID int) (window *TmuxWindow, fallback *TmuxWindow) {
	for _, candidate := range windows {
		if candidate == nil {
			continue
		}
		if fallback == nil {
			fallback = candidate
		}
		if candidate.ID == activeWindowID {
			window = candidate
			break
		}
	}
	return window, fallback
}

// findWindowByIDGlobalLocked searches every session for a globally unique
// window ID and returns both the window and its owning session.
//
// REQUIRES: m.mu held by the caller (RLock or Lock).
func (m *SessionManager) findWindowByIDGlobalLocked(windowID int) (*TmuxWindow, *TmuxSession) {
	for _, session := range m.sessions {
		if session == nil {
			continue
		}
		window, _ := findWindowByID(session.Windows, windowID)
		if window != nil {
			return window, session
		}
	}
	return nil, nil
}

func resolveWindowByTargetID(windows []*TmuxWindow, windowPart string) (*TmuxWindow, error) {
	windowPart = strings.TrimSpace(windowPart)
	if windowID, isWindowTarget, err := parseWindowIDTarget(windowPart); isWindowTarget {
		if err != nil {
			return nil, err
		}
		window, _ := findWindowByID(windows, windowID)
		if window == nil {
			return nil, fmt.Errorf("window id not found: %d", windowID)
		}
		return window, nil
	}

	windowIdx, err := strconv.Atoi(windowPart)
	if err != nil || windowIdx < 0 {
		return nil, fmt.Errorf("invalid window index: %s", windowPart)
	}
	if windowIdx >= len(windows) {
		return nil, fmt.Errorf("window index out of range: %d", windowIdx)
	}
	window := windows[windowIdx]
	if window == nil {
		return nil, fmt.Errorf("window at index %d is nil", windowIdx)
	}
	return window, nil
}

// activeWindowInSessionLocked resolves the active window under exclusive Lock.
//
// S-9: When ActiveWindowID is stale and auto-repaired to a fallback window,
// a debug log is emitted to aid investigation of stale-state scenarios.
//
// REQUIRES: m.mu must be held for writing.
func (m *SessionManager) activeWindowInSessionLocked(session *TmuxSession) *TmuxWindow {
	if session == nil || len(session.Windows) == 0 {
		return nil
	}
	activeWindow, fallback := findWindowByID(session.Windows, session.ActiveWindowID)
	if activeWindow != nil {
		return activeWindow
	}
	if fallback != nil && session.ActiveWindowID != fallback.ID {
		slog.Debug("[DEBUG-SESSION] activeWindowInSessionLocked: auto-repaired stale ActiveWindowID",
			"session", session.Name,
			"staleID", session.ActiveWindowID,
			"repairedID", fallback.ID,
		)
		session.ActiveWindowID = fallback.ID
		m.markStateMutationLocked()
	}
	return fallback
}

func activePaneInSession(session *TmuxSession) (*TmuxPane, error) {
	activeWindow := activeWindowInSession(session)
	if activeWindow == nil {
		return nil, errors.New("session has no windows")
	}
	return activePaneInWindow(activeWindow)
}

// activeWindowInSession is a read-only helper for already-cloned snapshots.
// It does not auto-repair session.ActiveWindowID.
func activeWindowInSession(session *TmuxSession) *TmuxWindow {
	if session == nil || len(session.Windows) == 0 {
		return nil
	}
	activeWindow, fallback := findWindowByID(session.Windows, session.ActiveWindowID)
	if activeWindow != nil {
		return activeWindow
	}
	return fallback
}

func activePaneInWindow(window *TmuxWindow) (*TmuxPane, error) {
	if window == nil || len(window.Panes) == 0 {
		return nil, errors.New("window has no panes")
	}
	if window.ActivePN >= 0 && window.ActivePN < len(window.Panes) {
		pane := window.Panes[window.ActivePN]
		if pane == nil {
			return nil, fmt.Errorf("active pane at index %d is nil", window.ActivePN)
		}
		return pane, nil
	}
	pane := window.Panes[0]
	if pane == nil {
		return nil, errors.New("first pane in window is nil")
	}
	return pane, nil
}
