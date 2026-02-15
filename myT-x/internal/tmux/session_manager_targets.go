package tmux

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ResolveTarget parses a tmux target and returns a pane.
func (m *SessionManager) ResolveTarget(target string, callerPaneID int) (*TmuxPane, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	target = strings.TrimSpace(target)
	if target == "" {
		if callerPaneID >= 0 {
			if pane, ok := m.panes[callerPaneID]; ok {
				return pane, nil
			}
		}
		return m.defaultPaneLocked()
	}

	if strings.HasPrefix(target, "%") {
		id, err := parsePaneID(target)
		if err != nil {
			return nil, err
		}
		pane, ok := m.panes[id]
		if !ok {
			return nil, fmt.Errorf("pane not found: %s", target)
		}
		return pane, nil
	}

	sessionName, remainder, hasColon := strings.Cut(target, ":")
	if !hasColon {
		session := m.sessions[sessionName]
		if session == nil {
			return nil, fmt.Errorf("session not found: %s", sessionName)
		}
		return activePaneInSession(session)
	}

	session := m.sessions[sessionName]
	if session == nil {
		return nil, fmt.Errorf("session not found: %s", sessionName)
	}
	if strings.TrimSpace(remainder) == "" {
		return activePaneInSession(session)
	}

	windowPart, panePart, hasPane := strings.Cut(remainder, ".")
	windowIdx, err := strconv.Atoi(windowPart)
	if err != nil {
		return nil, fmt.Errorf("invalid window index: %s", windowPart)
	}
	if windowIdx < 0 || windowIdx >= len(session.Windows) {
		return nil, fmt.Errorf("window index out of range: %d", windowIdx)
	}
	window := session.Windows[windowIdx]

	if !hasPane || strings.TrimSpace(panePart) == "" {
		return activePaneInWindow(window)
	}

	paneIdx, err := strconv.Atoi(panePart)
	if err != nil {
		return nil, fmt.Errorf("invalid pane index: %s", panePart)
	}
	if paneIdx < 0 || paneIdx >= len(window.Panes) {
		return nil, fmt.Errorf("pane index out of range: %d", paneIdx)
	}
	return window.Panes[paneIdx], nil
}

// ResolveSessionTarget resolves a session from a target-like string.
func (m *SessionManager) ResolveSessionTarget(target string) (*TmuxSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

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
		if !ok || pane.Window == nil || pane.Window.Session == nil {
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

func (m *SessionManager) defaultPaneLocked() (*TmuxPane, error) {
	if len(m.sessions) == 0 {
		return nil, errors.New("no sessions")
	}
	sessions := make([]*TmuxSession, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].ID < sessions[j].ID })
	return activePaneInSession(sessions[0])
}

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
		if !ok || pane.Window == nil || pane.Window.Session == nil {
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

func activePaneInSession(session *TmuxSession) (*TmuxPane, error) {
	if session == nil || len(session.Windows) == 0 {
		return nil, errors.New("session has no windows")
	}
	return activePaneInWindow(session.Windows[0])
}

func activePaneInWindow(window *TmuxWindow) (*TmuxPane, error) {
	if window == nil || len(window.Panes) == 0 {
		return nil, errors.New("window has no panes")
	}
	if window.ActivePN >= 0 && window.ActivePN < len(window.Panes) {
		return window.Panes[window.ActivePN], nil
	}
	return window.Panes[0], nil
}
