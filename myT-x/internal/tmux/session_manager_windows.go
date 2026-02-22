package tmux

// NOTE(1-window model): AddWindow and WindowIndexInSession have been deliberately
// removed as part of the 1-session-per-window architecture refactor.
//
// In this model, windows are created exclusively via CreateSession (one window per
// session). The multi-window API surface (AddWindow, WindowIndexInSession) is no
// longer needed because each session owns exactly one window.
//
// See handleNewWindow in command_router_handlers_window.go for the current flow.
// If multi-window support is re-enabled in the future, restore AddWindow and
// WindowIndexInSession from git history (pre-1-window-model commits).

import (
	"fmt"
	"strings"
)

// removeWindowResult holds the result of removeWindowAtIndexLocked.
// Fields are documented to make ownership contracts explicit.
type removeWindowResult struct {
	// RemovedPanes contains the panes that were removed from the window.
	// OWNERSHIP CONTRACT: The caller receives exclusive ownership of these panes
	// and MUST call Terminal.Close() on each non-nil pane to release ConPTY handles.
	// Failing to Close causes resource leaks.
	RemovedPanes []*TmuxPane

	// SessionRemoved is true when the removed window was the last window in the
	// session, causing the entire session to be deleted.
	SessionRemoved bool

	// SurvivingWindowID is the ActiveWindowID of the session after removal.
	// Set to -1 when SessionRemoved is true or no surviving windows exist.
	//
	// NOTE: The zero value (0) is ambiguous — it could mean "window ID 0"
	// (valid, since window IDs start at 0) or "unset". Callers must check
	// SessionRemoved or compare against -1 to distinguish. Sentinel -1 is
	// used for the "no surviving window" case.
	SurvivingWindowID int
}

// RemoveWindowByID removes a window by stable window ID.
// Returns removeWindowResult containing removed panes for terminal cleanup, whether the
// session was removed, and the surviving window ID (-1 if session was removed).
func (m *SessionManager) RemoveWindowByID(sessionName string, windowID int) (removeWindowResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, err := m.getSessionByNameLocked(sessionName)
	if err != nil {
		return removeWindowResult{SurvivingWindowID: -1}, err
	}
	windowIdx := findWindowIndexByID(session.Windows, windowID)
	if windowIdx < 0 {
		return removeWindowResult{SurvivingWindowID: -1}, fmt.Errorf("window not found in session: %s", sessionName)
	}
	return m.removeWindowAtIndexLocked(session, windowIdx)
}

// findWindowIndexByID returns the slice index of the window with the given ID,
// or -1 if not found. Nil entries are safely skipped.
func findWindowIndexByID(windows []*TmuxWindow, id int) int {
	for i, window := range windows {
		if window != nil && window.ID == id {
			return i
		}
	}
	return -1
}

func fallbackWindowIDNearIndex(windows []*TmuxWindow, preferredIdx int) (int, bool) {
	if len(windows) == 0 {
		return 0, false
	}
	if preferredIdx < 0 {
		preferredIdx = 0
	}
	if preferredIdx >= len(windows) {
		preferredIdx = len(windows) - 1
	}
	for i := preferredIdx; i < len(windows); i++ {
		if window := windows[i]; window != nil {
			return window.ID, true
		}
	}
	for i := preferredIdx - 1; i >= 0; i-- {
		if window := windows[i]; window != nil {
			return window.ID, true
		}
	}
	return 0, false
}

// removeWindowAtIndexLocked removes a window by slice index from a session.
//
// Naming convention (I-11): Methods with a "Locked" suffix require the caller
// to hold m.mu (either RLock or Lock depending on mutation). This convention
// is used consistently across SessionManager:
//   - *Locked  = caller must hold m.mu (write lock)
//   - *RLocked = caller must hold m.mu.RLock (read lock)
//
// REQUIRES: m.mu must be held by the caller.
func (m *SessionManager) removeWindowAtIndexLocked(session *TmuxSession, windowIdx int) (removeWindowResult, error) {
	if windowIdx < 0 || windowIdx >= len(session.Windows) {
		return removeWindowResult{SurvivingWindowID: -1}, fmt.Errorf("window index out of range: %d", windowIdx)
	}

	window := session.Windows[windowIdx]
	// Always initialize to an empty slice (not nil) so that callers never
	// receive a nil RemovedPanes. While current consumers only range-iterate
	// (safe with nil), an empty slice is safer for any future JSON serialization
	// or len/nil-distinction checks.
	capHint := 0
	if window != nil {
		capHint = len(window.Panes)
	}
	removedPanes := make([]*TmuxPane, 0, capHint)
	if window != nil {
		removedPanes = append(removedPanes, window.Panes...)
	}

	// Unregister panes from the global pane map.
	// Terminal is intentionally NOT nil-ified here: the returned removedPanes
	// are exclusively owned by the caller for cleanup (Terminal.Close) outside
	// the session manager lock. The pane is unreachable through m.panes after
	// delete, preventing internal access to stale Terminal references.
	// Contrast with killPaneLocked which saves Terminal to closeTargets before
	// nil-ifying — here the entire pane pointer is returned for the same purpose.
	for _, pane := range removedPanes {
		if pane == nil {
			continue
		}
		delete(m.panes, pane.ID)
	}

	// Remove window from session's window slice.
	session.Windows = append(session.Windows[:windowIdx], session.Windows[windowIdx+1:]...)

	// If that was the last window, remove the session entirely.
	if len(session.Windows) == 0 {
		delete(m.sessions, session.Name)
		m.markSessionMapMutationLocked()
		return removeWindowResult{
			RemovedPanes:      removedPanes,
			SessionRemoved:    true,
			SurvivingWindowID: -1,
		}, nil
	}

	// Keep session active window reference on a surviving window.
	// activeWindow is nil when the removed window was the active one
	// (its ID is no longer present in session.Windows after the splice above).
	// In that case, pick the nearest surviving window as the new active target.
	activeWindow, _ := findWindowByID(session.Windows, session.ActiveWindowID)
	if activeWindow == nil {
		if fallbackWindowID, ok := fallbackWindowIDNearIndex(session.Windows, windowIdx); ok {
			session.ActiveWindowID = fallbackWindowID
		}
	}

	m.markTopologyMutationLocked()
	return removeWindowResult{
		RemovedPanes:      removedPanes,
		SessionRemoved:    false,
		SurvivingWindowID: session.ActiveWindowID,
	}, nil
}

// RenameWindowByID changes the name of a window identified by stable window ID.
// Returns the resolved window index for event payload compatibility.
//
// NOTE(1-window model): WindowIndexInSession was removed because window lookup
// is now exclusively by stable ID (not slice index). However, this method still
// returns the index because the tmux event protocol (window-renamed) requires a
// window index in its payload. The index is computed locally from the session's
// Windows slice and is valid only within the current lock scope.
func (m *SessionManager) RenameWindowByID(sessionName string, windowID int, newName string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, err := m.getSessionByNameLocked(sessionName)
	if err != nil {
		return 0, err
	}
	windowIdx := findWindowIndexByID(session.Windows, windowID)
	if windowIdx < 0 {
		return 0, fmt.Errorf("window not found in session: %s", sessionName)
	}
	if err := m.renameWindowByIndexLocked(session, windowIdx, newName); err != nil {
		return 0, err
	}
	return windowIdx, nil
}

// REQUIRES: m.mu must be held by the caller.
func (m *SessionManager) renameWindowByIndexLocked(session *TmuxSession, windowIdx int, newName string) error {
	if windowIdx < 0 || windowIdx >= len(session.Windows) {
		return fmt.Errorf("window index out of range: %d", windowIdx)
	}

	newName = strings.TrimSpace(newName)
	if newName == "" {
		return fmt.Errorf("new window name cannot be empty")
	}

	window := session.Windows[windowIdx]
	if window == nil {
		return fmt.Errorf("window at index %d is nil", windowIdx)
	}
	window.Name = newName
	m.markStateMutationLocked()
	return nil
}
