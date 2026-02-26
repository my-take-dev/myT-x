package tmux

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

type terminalCloser interface {
	Close() error
}

// SwapPanes swaps two panes in the same window and updates layout references.
func (m *SessionManager) SwapPanes(sourcePaneID string, targetPaneID string) (string, error) {
	sourceID, err := parsePaneID(strings.TrimSpace(sourcePaneID))
	if err != nil {
		return "", err
	}
	targetID, err := parsePaneID(strings.TrimSpace(targetPaneID))
	if err != nil {
		return "", err
	}
	if sourceID == targetID {
		return "", errors.New("source and target pane are identical")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	source := m.panes[sourceID]
	target := m.panes[targetID]
	if source == nil {
		return "", fmt.Errorf("pane not found: %s", sourcePaneID)
	}
	if target == nil {
		return "", fmt.Errorf("pane not found: %s", targetPaneID)
	}
	if source.Window == nil || target.Window == nil || source.Window != target.Window || source.Window.Session == nil {
		return "", errors.New("swap requires panes in the same window")
	}

	window := source.Window
	sourceIdx := -1
	targetIdx := -1
	for i, pane := range window.Panes {
		if pane == nil {
			continue
		}
		switch pane.ID {
		case sourceID:
			sourceIdx = i
		case targetID:
			targetIdx = i
		}
	}
	if sourceIdx < 0 || targetIdx < 0 {
		return "", errors.New("pane not found in window")
	}

	window.Panes[sourceIdx], window.Panes[targetIdx] = window.Panes[targetIdx], window.Panes[sourceIdx]
	for idx, pane := range window.Panes {
		if pane == nil {
			continue
		}
		pane.Index = idx
		if pane.Active {
			window.ActivePN = idx
		}
	}
	nextLayout := swapPaneIDsInLayout(window.Layout, sourceID, targetID)
	if nextLayout == nil {
		nextLayout = rebuildLayoutFromPaneOrder(window.Panes)
	}
	window.Layout = nextLayout
	m.markTopologyMutationLocked()
	return window.Session.Name, nil
}

// killPaneResult holds the results from the lock-protected portion of KillPane.
type killPaneResult struct {
	sessionName       string
	removedSession    bool
	closeTargets      []terminalCloser
	removedFromWindow bool
}

// killPaneLocked performs the lock-protected portion of KillPane.
// Uses defer to guarantee lock release even on panic.
// When the session is deleted, defensively cleans up all remaining panes
// that might still be tracked in m.panes for that session.
func (m *SessionManager) killPaneLocked(id int, paneIDStr string) (killPaneResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result killPaneResult

	pane, ok := m.panes[id]
	if !ok || pane == nil {
		return result, fmt.Errorf("pane not found: %s", paneIDStr)
	}
	window := pane.Window
	if window == nil || window.Session == nil {
		return result, errors.New("pane has invalid parent")
	}
	session := window.Session
	result.sessionName = session.Name

	if pane.Terminal != nil {
		result.closeTargets = append(result.closeTargets, pane.Terminal)
		pane.Terminal = nil
	}
	delete(m.panes, id)

	nextWindows := make([]*TmuxWindow, 0, len(session.Windows))
	// removedWindowIdx is a spatial hint — the original session.Windows slice
	// index of the window that contained the killed pane.  It is NOT a stable
	// window ID; it is used only as a proximity hint when choosing a fallback
	// active window after pane removal causes the active window to become
	// empty or its ID to be removed.  fallbackWindowIDNearIndex picks the
	// closest surviving window by index, preserving spatial locality.
	removedWindowIdx := -1
	for windowIdx, sessionWindow := range session.Windows {
		if sessionWindow == nil {
			continue
		}
		nextPanes := make([]*TmuxPane, 0, len(sessionWindow.Panes))
		removedFromWindow := false
		for _, candidate := range sessionWindow.Panes {
			if candidate == nil {
				continue
			}
			if candidate.ID == pane.ID {
				removedFromWindow = true
				continue
			}
			nextPanes = append(nextPanes, candidate)
		}
		if !removedFromWindow {
			nextWindows = append(nextWindows, sessionWindow)
			continue
		}
		result.removedFromWindow = true
		removedWindowIdx = windowIdx

		sessionWindow.Panes = nextPanes
		for idx, candidate := range sessionWindow.Panes {
			if candidate == nil {
				continue
			}
			candidate.Index = idx
		}
		if len(sessionWindow.Panes) == 0 {
			continue
		}
		if sessionWindow.ActivePN < 0 || sessionWindow.ActivePN >= len(sessionWindow.Panes) {
			sessionWindow.ActivePN = 0
		}
		for i, candidate := range sessionWindow.Panes {
			if candidate == nil {
				continue
			}
			candidate.Active = i == sessionWindow.ActivePN
		}
		if nextLayout, removed := removePaneFromLayout(sessionWindow.Layout, pane.ID); removed && nextLayout != nil {
			sessionWindow.Layout = nextLayout
		} else {
			// Fallback when layout tree is already inconsistent with pane list.
			sessionWindow.Layout = rebuildLayoutFromPaneOrder(sessionWindow.Panes)
		}
		nextWindows = append(nextWindows, sessionWindow)
	}

	session.Windows = nextWindows
	if len(session.Windows) == 0 {
		delete(m.sessions, session.Name)
		result.removedSession = true

		// Defensive cleanup: when deleting the entire session, collect
		// terminals from any remaining panes that are still tracked in
		// m.panes for this session. In normal flow the killed pane was
		// the only one, but this protects against data inconsistency.
		for pid, orphan := range m.panes {
			if orphan == nil || orphan.Window == nil || orphan.Window.Session != session {
				continue
			}
			if orphan.Terminal != nil {
				result.closeTargets = append(result.closeTargets, orphan.Terminal)
				orphan.Terminal = nil
			}
			delete(m.panes, pid)
			slog.Warn("[WARN-PANE] KillPane: cleaned up orphaned pane during session deletion",
				"paneId", orphan.IDString(),
				"session", result.sessionName,
			)
		}
		m.markSessionMapMutationLocked()
	} else {
		activeWindow, _ := findWindowByID(session.Windows, session.ActiveWindowID)
		if activeWindow == nil {
			preferredIdx := max(removedWindowIdx, 0)
			if fallbackWindowID, ok := fallbackWindowIDNearIndex(session.Windows, preferredIdx); ok {
				session.ActiveWindowID = fallbackWindowID
			}
		}
		m.markTopologyMutationLocked()
	}

	return result, nil
}

// KillPane closes and removes one pane.
func (m *SessionManager) KillPane(paneID string) (sessionName string, removedSession bool, err error) {
	id, err := parsePaneID(strings.TrimSpace(paneID))
	if err != nil {
		return "", false, err
	}

	result, err := m.killPaneLocked(id, paneID)
	if err != nil {
		return "", false, err
	}

	// Close terminals outside lock to avoid blocking other SessionManager operations.
	for _, ct := range result.closeTargets {
		if closeErr := ct.Close(); closeErr != nil {
			slog.Warn("[WARN-PANE] KillPane terminal close failed",
				"paneId", fmt.Sprintf("%%%d", id),
				"session", result.sessionName,
				"error", closeErr,
			)
		}
	}
	if !result.removedFromWindow {
		slog.Warn("[WARN-PANE] KillPane removed pane from map but not from any window",
			"paneId", fmt.Sprintf("%%%d", id),
			"session", result.sessionName,
		)
	}
	return result.sessionName, result.removedSession, nil
}

func rebuildLayoutFromPaneOrder(panes []*TmuxPane) *LayoutNode {
	var root *LayoutNode
	for _, p := range panes {
		if p == nil {
			continue
		}
		leaf := newLeafLayout(p.ID)
		if root == nil {
			root = leaf
		} else {
			root = &LayoutNode{
				Type:      LayoutSplit,
				Direction: SplitHorizontal,
				Ratio:     0.5,
				Children: [2]*LayoutNode{
					root,
					leaf,
				},
			}
		}
	}
	return root
}
