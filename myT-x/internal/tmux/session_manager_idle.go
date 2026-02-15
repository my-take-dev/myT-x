package tmux

import (
	"strings"
)

// UpdateActivityByPaneID updates a session's activity timestamp by pane id (%N).
// It returns true when an idle session moved back to active.
func (m *SessionManager) UpdateActivityByPaneID(paneID string) bool {
	id, err := parsePaneID(strings.TrimSpace(paneID))
	if err != nil {
		return false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	pane := m.panes[id]
	if pane == nil || pane.Window == nil || pane.Window.Session == nil {
		return false
	}

	session := pane.Window.Session
	session.LastActivity = m.now()
	if !session.IsIdle {
		return false
	}
	session.IsIdle = false
	return true
}

// CheckIdleState evaluates all sessions and returns true when any idle state changed.
func (m *SessionManager) CheckIdleState() bool {
	now := m.now()

	m.mu.Lock()
	defer m.mu.Unlock()

	changed := false
	for _, session := range m.sessions {
		if session == nil {
			continue
		}

		last := session.LastActivity
		if last.IsZero() {
			last = session.CreatedAt
		}
		idle := now.Sub(last) >= m.idleThreshold
		if idle == session.IsIdle {
			continue
		}
		session.IsIdle = idle
		changed = true
	}

	return changed
}
