package tmux

import (
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
)

// ResolveTarget parses a tmux target and returns a pane.
//
// IMPORTANT: The returned *TmuxPane is a live internal pointer. Callers must
// only read scalar fields (ID, IDString(), Width, Height) immediately after
// the call. Do not cache the pointer or dereference after subsequent lock
// acquisitions, as the pane may be removed concurrently. For long-lived
// access patterns, use GetPaneContextSnapshot instead.
//
// Lock strategy (I-3): Uses RLock for the common read-only path. If
// activeWindowInSessionLocked detects a stale ActiveWindowID that needs
// auto-repair, the method upgrades to a write Lock with double-check
// locking to avoid serialising every high-frequency command call.
func (m *SessionManager) ResolveTarget(target string, callerPaneID int) (*TmuxPane, error) {
	// Fast path: try with RLock first.
	pane, needsRepair, err := m.resolveTargetRLocked(target, callerPaneID)
	if err != nil {
		return nil, err
	}
	if !needsRepair {
		return pane, nil
	}

	// Slow path: auto-repair requires write Lock.
	// Double-check locking: re-resolve under exclusive lock because state
	// may have changed between RLock release and Lock acquisition.
	return m.resolveTargetWriteLocked(target, callerPaneID)
}

// resolveTargetCore contains the shared target-resolution logic used by both
// resolveTargetRLocked and resolveTargetWriteLocked.
//
// C-01: Extracted to eliminate ~50 lines of duplicated resolution logic.
// Bug fixes applied here automatically propagate to both the RLock and Lock
// code paths.
//
// Return values:
//   - pane: the resolved pane (may be non-nil even when needsSessionResolve is true).
//     When needsSessionResolve is true, pane is always nil; the caller must resolve
//     the active pane via activePaneInSession{R,}Locked or defaultPane{R,}Locked.
//   - needsSessionResolve: when true, the target resolved to a session-level
//     lookup (empty target default, bare session name, or session: with empty
//     remainder) and the caller must delegate to activePaneInSession{R,}Locked.
//   - session: the resolved session when needsSessionResolve is true. May be nil
//     when the target is empty and callerPaneID is negative (no explicit session);
//     in that case, the caller falls back to defaultPane{R,}Locked which resolves
//     the first session's active pane.
//   - error: non-nil when resolution fails definitively.
//
// REQUIRES: m.mu must be held by the caller in some mode (RLock or Lock).
func (m *SessionManager) resolveTargetCore(target string, callerPaneID int) (pane *TmuxPane, needsSessionResolve bool, session *TmuxSession, err error) {
	target = strings.TrimSpace(target)
	if target == "" {
		if callerPaneID >= 0 {
			if p, ok := m.panes[callerPaneID]; ok {
				if p == nil {
					return nil, false, nil, fmt.Errorf("pane is nil: %%%d", callerPaneID)
				}
				return p, false, nil, nil
			}
		}
		// Caller must resolve via defaultPane{R,}Locked.
		return nil, true, nil, nil
	}

	if strings.HasPrefix(target, "%") {
		id, parseErr := parsePaneID(target)
		if parseErr != nil {
			return nil, false, nil, parseErr
		}
		p, ok := m.panes[id]
		if !ok || p == nil {
			return nil, false, nil, fmt.Errorf("pane not found: %s", target)
		}
		return p, false, nil, nil
	}

	sessionName, rem, hasColon := strings.Cut(target, ":")
	sess := m.sessions[sessionName]
	if !hasColon {
		if sess == nil {
			return nil, false, nil, fmt.Errorf("session not found: %s", sessionName)
		}
		return nil, true, sess, nil
	}

	if sess == nil {
		return nil, false, nil, fmt.Errorf("session not found: %s", sessionName)
	}
	if strings.TrimSpace(rem) == "" {
		return nil, true, sess, nil
	}

	// Window:pane target — delegate to resolveWindowPaneTarget.
	p, _, resolveErr := m.resolveWindowPaneTarget(sess, target, rem)
	return p, false, nil, resolveErr
}

// resolveTargetRLocked resolves a target under RLock.
// Returns (pane, needsRepair, error).  When needsRepair is true, the caller
// must re-resolve under a write Lock because activeWindowInSession detected
// a stale ActiveWindowID that requires mutation.
func (m *SessionManager) resolveTargetRLocked(target string, callerPaneID int) (*TmuxPane, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pane, needsSessionResolve, session, err := m.resolveTargetCore(target, callerPaneID)
	if err != nil {
		return nil, false, err
	}
	if !needsSessionResolve {
		return pane, false, nil
	}

	// Session-level resolution: requires activePaneInSession variant.
	if session != nil {
		return m.activePaneInSessionRLocked(session)
	}
	// Empty target with no callerPaneID match — fall back to default pane.
	return m.defaultPaneRLocked()
}

// resolveWindowPaneTarget resolves a "session:window.pane" target under an
// already-held lock (either RLock or Lock).
//
// REQUIRES: m.mu must be held by the caller in some mode (RLock or Lock).
func (m *SessionManager) resolveWindowPaneTarget(session *TmuxSession, target, remainder string) (*TmuxPane, bool, error) {
	windowPart, panePart, hasPane := strings.Cut(remainder, ".")
	windowPart = strings.TrimSpace(windowPart)
	var window *TmuxWindow
	if after, ok := strings.CutPrefix(windowPart, "@"); ok {
		windowIDText := strings.TrimSpace(after)
		windowID, err := strconv.Atoi(windowIDText)
		if err != nil || windowID < 0 {
			return nil, false, fmt.Errorf("invalid window id: %s", windowPart)
		}
		window, _ = findWindowByID(session.Windows, windowID)
		if window == nil {
			return nil, false, fmt.Errorf("window id not found: %d", windowID)
		}
	} else {
		windowIdx, err := strconv.Atoi(windowPart)
		if err != nil {
			return nil, false, fmt.Errorf("invalid window index: %s", windowPart)
		}
		if windowIdx < 0 || windowIdx >= len(session.Windows) {
			return nil, false, fmt.Errorf("window index out of range: %d", windowIdx)
		}
		window = session.Windows[windowIdx]
	}
	if window == nil {
		return nil, false, fmt.Errorf("window not found in target: %s", target)
	}

	if !hasPane || strings.TrimSpace(panePart) == "" {
		pane, err := activePaneInWindow(window)
		return pane, false, err
	}

	paneIdx, err := strconv.Atoi(panePart)
	if err != nil {
		return nil, false, fmt.Errorf("invalid pane index: %s", panePart)
	}
	if paneIdx < 0 || paneIdx >= len(window.Panes) {
		return nil, false, fmt.Errorf("pane index out of range: %d", paneIdx)
	}
	pane := window.Panes[paneIdx]
	if pane == nil {
		return nil, false, fmt.Errorf("pane at index %d is nil", paneIdx)
	}
	return pane, false, nil
}

// resolveTargetWriteLocked re-resolves a target under exclusive Lock.
// Called only when the RLock fast path detected that auto-repair is needed.
//
// C-01: Delegates to resolveTargetCore for shared resolution logic, then
// uses the write-lock variants (activePaneInSessionLocked, defaultPaneLocked)
// for session-level resolution that may auto-repair stale state.
func (m *SessionManager) resolveTargetWriteLocked(target string, callerPaneID int) (*TmuxPane, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pane, needsSessionResolve, session, err := m.resolveTargetCore(target, callerPaneID)
	if err != nil {
		return nil, err
	}
	if !needsSessionResolve {
		return pane, nil
	}

	// Session-level resolution: uses write-lock variant that can auto-repair
	// stale ActiveWindowID.
	if session != nil {
		return m.activePaneInSessionLocked(session)
	}
	// Empty target with no callerPaneID match — fall back to default pane.
	return m.defaultPaneLocked()
}

// DirectionalPaneDirection represents a directional pane navigation request.
type DirectionalPaneDirection int

const (
	// DirNone is the zero value for DirectionalPaneDirection.
	// An uninitialised variable or unrecognised direction resolves to DirNone,
	// which causes ResolveDirectionalPane to return the current pane unchanged.
	DirNone DirectionalPaneDirection = iota
	// DirPrev navigates to the previous pane (left/up).
	DirPrev
	// DirNext navigates to the next pane (right/down).
	DirNext
)

// ResolveDirectionalPane resolves the current pane and navigates to a neighbouring
// pane in the specified direction, all under a single lock acquisition.
// This eliminates the TOCTOU race of resolving current pane, listing window panes,
// and re-resolving the candidate in three independent lock scopes.
func (m *SessionManager) ResolveDirectionalPane(callerPaneID int, direction DirectionalPaneDirection) (*TmuxPane, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Step 1: Resolve current pane.
	var current *TmuxPane
	if callerPaneID >= 0 {
		var ok bool
		current, ok = m.panes[callerPaneID]
		if !ok || current == nil {
			return nil, fmt.Errorf("caller pane not found: %%%d", callerPaneID)
		}
	} else {
		pane, err := m.defaultPaneLocked()
		if err != nil {
			return nil, err
		}
		current = pane
	}

	if current.Window == nil {
		if direction == DirNone {
			return current, nil
		}
		return nil, errors.New("current pane has no window")
	}

	if direction == DirNone {
		return current, nil
	}

	// Step 2: List panes in the same window.
	panes := current.Window.Panes
	if len(panes) == 0 {
		return nil, errors.New("window has no panes")
	}

	// Step 3: Find current pane index and navigate.
	idx := -1
	for i, p := range panes {
		if p != nil && p.ID == current.ID {
			idx = i
			break
		}
	}
	if idx < 0 {
		// Pane was removed concurrently (shouldn't happen under lock, but defensive).
		if panes[0] == nil {
			return nil, errors.New("first pane in window is nil")
		}
		return panes[0], nil
	}

	switch direction {
	case DirPrev:
		idx--
	case DirNext:
		idx++
	}
	// COMPATIBILITY DEVIATION: Real tmux wraps around when navigating past
	// the first or last pane (e.g., DirPrev on pane[0] -> pane[len-1]).
	// This implementation clamps to the boundary, returning the same pane
	// when at an edge. Wrap-around is intentionally not implemented because
	// the GUI navigation model does not require it.
	if idx < 0 {
		idx = 0
	}
	if idx >= len(panes) {
		idx = len(panes) - 1
	}

	// Skip nil panes in the navigation direction, falling back to current pane
	// if no valid pane is found in the chosen direction.
	candidate := panes[idx]
	if candidate == nil {
		switch direction {
		case DirNext:
			for i := idx + 1; i < len(panes); i++ {
				if panes[i] != nil {
					return panes[i], nil
				}
			}
		case DirPrev:
			for i := idx - 1; i >= 0; i-- {
				if panes[i] != nil {
					return panes[i], nil
				}
			}
		}
		// No valid pane found in the chosen direction; return current pane.
		return current, nil
	}
	return candidate, nil
}

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
	sessionName, _, _ := strings.Cut(target, ":")
	session, ok := m.sessions[sessionName]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionName)
	}
	return session, nil
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
