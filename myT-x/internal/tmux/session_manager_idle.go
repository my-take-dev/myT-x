package tmux

import (
	"strings"
	"time"
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
	m.markStateMutationLocked()
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

	if changed {
		m.markStateMutationLocked()
	}
	return changed
}

// RecommendedIdleCheckInterval returns an adaptive polling interval for idle checks.
func (m *SessionManager) RecommendedIdleCheckInterval() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.sessions) == 0 {
		return 5 * time.Second
	}
	allIdle := true
	for _, session := range m.sessions {
		if session == nil {
			continue
		}
		if !session.IsIdle {
			allIdle = false
			break
		}
	}
	if allIdle {
		return 5 * time.Second
	}
	return time.Second
}
