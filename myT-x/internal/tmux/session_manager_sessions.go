package tmux

import (
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
)

// closeLocked collects all panes under lock and clears internal state.
// Uses defer to guarantee lock release even on panic.
func (m *SessionManager) closeLocked() []*TmuxPane {
	m.mu.Lock()
	defer m.mu.Unlock()

	panes := make([]*TmuxPane, 0, len(m.panes))
	for _, pane := range m.panes {
		panes = append(panes, pane)
	}
	m.sessions = map[string]*TmuxSession{}
	m.panes = map[int]*TmuxPane{}
	m.sortedSessionNames = nil
	m.markSessionMapMutationLocked()
	return panes
}

// Close shuts down all pane terminals.
func (m *SessionManager) Close() {
	panes := m.closeLocked()

	closeErrs := make([]error, 0)
	for _, pane := range panes {
		if pane == nil || pane.Terminal == nil {
			continue
		}
		if err := pane.Terminal.Close(); err != nil {
			closeErrs = append(closeErrs, fmt.Errorf("pane %%%d: %w", pane.ID, err))
		}
	}
	if len(closeErrs) > 0 {
		slog.Warn("[WARN-SESSION] SessionManager.Close terminal close errors", "error", errors.Join(closeErrs...))
	}
}

// CreateSession creates a session with one window and one pane.
func (m *SessionManager) CreateSession(name string, windowName string, width, height int) (*TmuxSession, *TmuxPane, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	name = strings.TrimSpace(name)
	if name == "" {
		name = m.nextAutoSessionNameLocked()
	}
	if _, exists := m.sessions[name]; exists {
		return nil, nil, fmt.Errorf("session already exists: %s", name)
	}
	if width <= 0 {
		width = DefaultTerminalCols
	}
	if height <= 0 {
		height = DefaultTerminalRows
	}
	if strings.TrimSpace(windowName) == "" {
		windowName = "0"
	}
	now := m.now()

	session := &TmuxSession{
		ID:           m.nextSessionID,
		Name:         name,
		CreatedAt:    now,
		LastActivity: now,
		Env:          map[string]string{},
	}
	m.nextSessionID++

	window := &TmuxWindow{
		ID:       m.nextWindowID,
		Name:     windowName,
		Layout:   nil,
		ActivePN: 0,
		Session:  session,
	}
	session.ActiveWindowID = window.ID
	m.nextWindowID++
	pane := &TmuxPane{
		ID:       m.nextPaneID,
		idString: fmt.Sprintf("%%%d", m.nextPaneID),
		Index:    0,
		Active:   true,
		Width:    width,
		Height:   height,
		Env:      map[string]string{},
		Window:   window,
	}
	m.nextPaneID++

	window.Panes = []*TmuxPane{pane}
	window.Layout = newLeafLayout(pane.ID)
	session.Windows = []*TmuxWindow{window}

	m.sessions[session.Name] = session
	m.panes[pane.ID] = pane
	m.markSessionMapMutationLocked()
	return session, pane, nil
}

func (m *SessionManager) nextAutoSessionNameLocked() string {
	for i := 0; ; i++ {
		name := strconv.Itoa(i)
		if _, exists := m.sessions[name]; !exists {
			return name
		}
	}
}

// RenameSession changes the name of an existing session.
func (m *SessionManager) RenameSession(oldName, newName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return fmt.Errorf("new session name cannot be empty")
	}
	if oldName == newName {
		return nil
	}

	session, ok := m.sessions[oldName]
	if !ok {
		return fmt.Errorf("session not found: %s", oldName)
	}
	if _, exists := m.sessions[newName]; exists {
		return fmt.Errorf("session already exists: %s", newName)
	}

	delete(m.sessions, oldName)
	session.Name = newName
	m.sessions[newName] = session
	m.markSessionMapMutationLocked()
	return nil
}

// removeSessionLocked performs the lock-protected portion of RemoveSession.
// Uses defer to guarantee lock release even on panic.
func (m *SessionManager) removeSessionLocked(name string) (*TmuxSession, []*TmuxPane, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessionName := parseSessionName(name)
	session, ok := m.sessions[sessionName]
	if !ok {
		return nil, nil, fmt.Errorf("session not found: %s", sessionName)
	}

	sessionCopy := cloneSessionForRead(session)
	panes := make([]*TmuxPane, 0)
	for _, window := range session.Windows {
		if window == nil {
			continue
		}
		for _, pane := range window.Panes {
			if pane == nil {
				continue
			}
			panes = append(panes, pane)
			delete(m.panes, pane.ID)
		}
	}
	delete(m.sessions, sessionName)
	m.markSessionMapMutationLocked()
	return sessionCopy, panes, nil
}

// RemoveSession closes terminals and removes session state.
func (m *SessionManager) RemoveSession(name string) (*TmuxSession, error) {
	sessionCopy, panes, err := m.removeSessionLocked(name)
	if err != nil {
		return nil, err
	}

	closeErrs := make([]error, 0)
	for _, pane := range panes {
		if pane == nil || pane.Terminal == nil {
			continue
		}
		if err := pane.Terminal.Close(); err != nil {
			closeErrs = append(closeErrs, fmt.Errorf("pane %%%d: %w", pane.ID, err))
		}
	}
	if len(closeErrs) > 0 {
		slog.Warn("[WARN-SESSION] RemoveSession terminal close errors",
			"session", sessionCopy.Name,
			"error", errors.Join(closeErrs...),
		)
	}
	return sessionCopy, nil
}

// HasSession checks whether a session exists.
func (m *SessionManager) HasSession(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sessionName := parseSessionName(name)
	_, ok := m.sessions[sessionName]
	return ok
}

// ListSessions returns all sessions sorted by id.
func (m *SessionManager) ListSessions() []*TmuxSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*TmuxSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, cloneSessionForRead(s))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// GetSession returns a session by name.
func (m *SessionManager) GetSession(name string) (*TmuxSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, ok := m.sessions[parseSessionName(name)]
	if !ok {
		return nil, false
	}
	return cloneSessionForRead(session), true
}

// cloneSessionForRead creates a deep copy of a session for safe read access
// outside of lock scope.
//
// S-45: Cloned panes intentionally have Terminal=nil. The Terminal field is
// an internal resource that must only be accessed under SessionManager.mu.
// Callers of cloneSessionForRead (GetSession, ListSessions, ResolveSessionTarget,
// RemoveSession) receive a snapshot that is safe to read and mutate without
// affecting internal state, but cannot perform I/O through the clone.
// Code paths that need Terminal access (e.g. emitLayoutChangedForSession)
// must look up panes by ID through SessionManager methods.
func cloneSessionForRead(session *TmuxSession) *TmuxSession {
	if session == nil {
		return nil
	}

	cloned := &TmuxSession{
		ID:             session.ID,
		Name:           session.Name,
		CreatedAt:      session.CreatedAt,
		LastActivity:   session.LastActivity,
		IsIdle:         session.IsIdle,
		Env:            copyEnvMap(session.Env),
		IsAgentTeam:    session.IsAgentTeam,
		RootPath:       session.RootPath,
		ActiveWindowID: session.ActiveWindowID,
		UseClaudeEnv:   copyBoolPtr(session.UseClaudeEnv),
		UsePaneEnv:     copyBoolPtr(session.UsePaneEnv),
	}
	if session.Worktree != nil {
		worktreeCopy := *session.Worktree
		cloned.Worktree = &worktreeCopy
	}

	// S-47: Use append-based construction to skip nil windows/panes cleanly,
	// producing a compact slice without nil holes that could cause index-based
	// access panics in callers.
	cloned.Windows = make([]*TmuxWindow, 0, len(session.Windows))
	for _, window := range session.Windows {
		if window == nil {
			continue
		}
		windowCopy := &TmuxWindow{
			ID:       window.ID,
			Name:     window.Name,
			Layout:   cloneLayout(window.Layout),
			ActivePN: 0, // Recalculated below after nil pane filtering.
			Session:  cloned,
		}
		windowCopy.Panes = make([]*TmuxPane, 0, len(window.Panes))
		for srcIdx, pane := range window.Panes {
			if pane == nil {
				continue
			}
			if srcIdx == window.ActivePN {
				windowCopy.ActivePN = len(windowCopy.Panes)
			}
			paneCopy := &TmuxPane{
				ID:       pane.ID,
				idString: pane.idString,
				Index:    pane.Index,
				Title:    pane.Title,
				Active:   pane.Active,
				Width:    pane.Width,
				Height:   pane.Height,
				Env:      copyEnvMap(pane.Env),
				Window:   windowCopy,
				// S-45: Terminal intentionally nil — see function doc.
			}
			windowCopy.Panes = append(windowCopy.Panes, paneCopy)
		}
		cloned.Windows = append(cloned.Windows, windowCopy)
	}
	return cloned
}
