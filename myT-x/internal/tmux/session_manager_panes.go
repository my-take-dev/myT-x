package tmux

import (
	"errors"
	"fmt"
)

// SplitPane splits target pane and returns new pane.
func (m *SessionManager) SplitPane(targetPaneID int, direction SplitDirection) (*TmuxPane, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	target, ok := m.panes[targetPaneID]
	if !ok || target == nil {
		return nil, fmt.Errorf("target pane not found: %%%d", targetPaneID)
	}

	window := target.Window
	if window == nil {
		return nil, errors.New("target pane has no parent window")
	}

	newPane := &TmuxPane{
		ID:       m.nextPaneID,
		idString: fmt.Sprintf("%%%d", m.nextPaneID),
		Index:    len(window.Panes),
		Active:   true,
		Width:    target.Width,
		Height:   target.Height,
		Env:      copyEnvMap(target.Env),
		Window:   window,
	}
	m.nextPaneID++

	target.Active = false
	window.ActivePN = newPane.Index
	window.Panes = append(window.Panes, newPane)

	nextLayout, ok := splitLayout(window.Layout, targetPaneID, direction, newPane.ID)
	if !ok {
		return nil, fmt.Errorf("layout update failed for pane: %%%d", targetPaneID)
	}
	window.Layout = nextLayout
	m.panes[newPane.ID] = newPane
	m.markTopologyMutationLocked()

	return newPane, nil
}

// SetActivePane marks one pane active in its window.
func (m *SessionManager) SetActivePane(paneID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pane, ok := m.panes[paneID]
	if !ok || pane == nil {
		return fmt.Errorf("pane not found: %%%d", paneID)
	}
	window := pane.Window
	if window == nil {
		return errors.New("pane has no parent window")
	}
	session := window.Session
	if session == nil {
		return errors.New("pane window has no parent session")
	}

	for _, p := range window.Panes {
		if p == nil {
			continue
		}
		p.Active = false
	}
	pane.Active = true
	window.ActivePN = pane.Index
	session.ActiveWindowID = window.ID
	// Active pane changes alter frontend-visible pane ordering/selection semantics.
	// We classify this as a topology mutation so Snapshot() consumers can coalesce
	// pane-state synchronization on TopologyGeneration.
	m.markTopologyMutationLocked()
	return nil
}

// ApplyLayoutPreset rearranges the window layout to match the given preset.
func (m *SessionManager) ApplyLayoutPreset(sessionName string, windowIdx int, preset LayoutPreset) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionName]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionName)
	}
	if windowIdx < 0 || windowIdx >= len(session.Windows) {
		return fmt.Errorf("window index out of range: %d", windowIdx)
	}
	window := session.Windows[windowIdx]
	if window == nil {
		return fmt.Errorf("window at index %d is nil", windowIdx)
	}
	return m.applyLayoutPresetToWindowLocked(window, preset)
}

// ApplyLayoutPresetByWindowID rearranges a window layout identified by stable
// window ID.
func (m *SessionManager) ApplyLayoutPresetByWindowID(sessionName string, windowID int, preset LayoutPreset) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionName]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionName)
	}
	for _, window := range session.Windows {
		if window == nil || window.ID != windowID {
			continue
		}
		return m.applyLayoutPresetToWindowLocked(window, preset)
	}
	return fmt.Errorf("window not found in session: %s", sessionName)
}

// ApplyLayoutPresetToActiveWindow atomically resolves the active window of a
// session and applies a layout preset. This eliminates the TOCTOU gap between
// reading ActiveWindowID from a snapshot clone and the actual preset application.
// If ActiveWindowID points to a deleted window, the first non-nil window is used
// as fallback and ActiveWindowID is repaired.
func (m *SessionManager) ApplyLayoutPresetToActiveWindow(sessionName string, preset LayoutPreset) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionName]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionName)
	}
	window := m.activeWindowInSessionLocked(session)
	if window == nil {
		return errors.New("session has no windows")
	}
	return m.applyLayoutPresetToWindowLocked(window, preset)
}

// REQUIRES: m.mu must be held by the caller.
func (m *SessionManager) applyLayoutPresetToWindowLocked(window *TmuxWindow, preset LayoutPreset) error {
	if window == nil {
		return errors.New("window is nil")
	}
	if len(window.Panes) == 0 {
		return errors.New("window has no panes")
	}
	paneIDs := make([]int, 0, len(window.Panes))
	for _, pane := range window.Panes {
		if pane == nil {
			continue
		}
		paneIDs = append(paneIDs, pane.ID)
	}
	if len(paneIDs) == 0 {
		return errors.New("window has no valid panes")
	}
	window.Layout = BuildPresetLayout(preset, paneIDs)
	m.markTopologyMutationLocked()
	return nil
}
