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
	if !ok {
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
	if !ok {
		return fmt.Errorf("pane not found: %%%d", paneID)
	}
	window := pane.Window
	if window == nil {
		return errors.New("pane has no parent window")
	}

	for _, p := range window.Panes {
		p.Active = false
	}
	pane.Active = true
	window.ActivePN = pane.Index
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
	if len(window.Panes) == 0 {
		return errors.New("window has no panes")
	}
	paneIDs := make([]int, len(window.Panes))
	for i, pane := range window.Panes {
		paneIDs[i] = pane.ID
	}
	window.Layout = BuildPresetLayout(preset, paneIDs)
	m.markTopologyMutationLocked()
	return nil
}
