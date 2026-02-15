package tmux

import (
	"fmt"
	"strings"

	"myT-x/internal/terminal"
)

// PaneContextSnapshot is a lock-safe snapshot of pane-owned session/window state.
type PaneContextSnapshot struct {
	SessionID   int
	SessionName string
	Layout      *LayoutNode
	Env         map[string]string
	Title       string
}

// parseSessionName extracts the session name from a target string.
// If name contains a colon, only the portion before it is used
// (e.g., "mysession:0" → "mysession").
func parseSessionName(name string) string {
	sessionName, _, _ := strings.Cut(strings.TrimSpace(name), ":")
	return sessionName
}

func normalizeSessionWorktreeInfo(info *SessionWorktreeInfo) *SessionWorktreeInfo {
	if info == nil {
		return nil
	}
	// Trim whitespace at the session boundary so IsEmpty() can remain a pure
	// zero-value check over normalized fields.
	normalized := &SessionWorktreeInfo{
		Path:       strings.TrimSpace(info.Path),
		RepoPath:   strings.TrimSpace(info.RepoPath),
		BranchName: strings.TrimSpace(info.BranchName),
		BaseBranch: strings.TrimSpace(info.BaseBranch),
		IsDetached: info.IsDetached,
	}
	if normalized.IsEmpty() {
		return nil
	}
	return normalized
}

// REQUIRES: m.mu must be held by the caller.
func (m *SessionManager) getSessionByNameLocked(name string) (*TmuxSession, error) {
	sessionName := parseSessionName(name)
	if sessionName == "" {
		return nil, fmt.Errorf("session name is required")
	}
	session, ok := m.sessions[sessionName]
	if !ok || session == nil {
		return nil, fmt.Errorf("session not found: %s", sessionName)
	}
	return session, nil
}

// GetSessionEnv returns a copy of environment variables for the named session.
// If name contains a colon, only the portion before it is used as the session
// lookup key (e.g., "mysession:0" resolves to "mysession").
func (m *SessionManager) GetSessionEnv(name string) (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, err := m.getSessionByNameLocked(name)
	if err != nil {
		return nil, err
	}
	return copyEnvMap(session.Env), nil
}

// SetWorktreeInfo sets worktree metadata on the named session.
// Passing nil clears worktree metadata.
// String fields are normalized with strings.TrimSpace before being stored, so
// whitespace-only values are treated as empty.
func (m *SessionManager) SetWorktreeInfo(name string, info *SessionWorktreeInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, err := m.getSessionByNameLocked(name)
	if err != nil {
		return err
	}
	normalized := normalizeSessionWorktreeInfo(info)
	if normalized == nil {
		session.Worktree = nil
		return nil
	}

	session.Worktree = normalized
	return nil
}

// GetWorktreeInfo returns worktree metadata for the named session.
// Returns (nil, nil) when the session exists but has no worktree metadata.
func (m *SessionManager) GetWorktreeInfo(name string) (*SessionWorktreeInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, err := m.getSessionByNameLocked(name)
	if err != nil {
		return nil, err
	}
	if session.Worktree == nil {
		return nil, nil
	}
	copied := *session.Worktree
	return &copied, nil
}

// SetRootPath stores the user-selected root directory for the named session.
func (m *SessionManager) SetRootPath(name, rootPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, err := m.getSessionByNameLocked(name)
	if err != nil {
		return err
	}
	session.RootPath = rootPath
	return nil
}

// SetAgentTeam updates the session's Agent Teams flag under lock.
func (m *SessionManager) SetAgentTeam(name string, isAgent bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, err := m.getSessionByNameLocked(name)
	if err != nil {
		return err
	}
	session.IsAgentTeam = isAgent
	return nil
}

// GetPaneEnv returns a copy of environment variables for the pane identified
// by paneID (format "%N"). The caller may safely mutate the returned map
// without affecting internal state.
func (m *SessionManager) GetPaneEnv(paneID string) (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id, err := parsePaneID(strings.TrimSpace(paneID))
	if err != nil {
		return nil, err
	}
	pane, ok := m.panes[id]
	if !ok || pane == nil {
		return nil, fmt.Errorf("pane not found: %%%d", id)
	}
	return copyEnvMap(pane.Env), nil
}

// SetPaneRuntime binds runtime terminal state for an existing pane under lock.
func (m *SessionManager) SetPaneRuntime(paneID int, term *terminal.Terminal, env map[string]string, inheritTitle string) error {
	if term == nil {
		return fmt.Errorf("pane runtime terminal is nil: %%%d", paneID)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	pane, ok := m.panes[paneID]
	if !ok || pane == nil {
		return fmt.Errorf("pane not found: %%%d", paneID)
	}
	pane.Terminal = term
	pane.Env = copyEnvMap(env)
	if strings.TrimSpace(pane.Title) == "" && strings.TrimSpace(inheritTitle) != "" {
		pane.Title = inheritTitle
	}
	return nil
}

// GetPaneContextSnapshot returns lock-safe pane/session/window context for paneID.
func (m *SessionManager) GetPaneContextSnapshot(paneID int) (PaneContextSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pane, ok := m.panes[paneID]
	if !ok || pane == nil || pane.Window == nil || pane.Window.Session == nil {
		return PaneContextSnapshot{}, fmt.Errorf("pane not found: %%%d", paneID)
	}
	return PaneContextSnapshot{
		SessionID:   pane.Window.Session.ID,
		SessionName: pane.Window.Session.Name,
		Layout:      cloneLayout(pane.Window.Layout),
		Env:         copyEnvMap(pane.Env),
		Title:       pane.Title,
	}, nil
}

// paneLayoutSnapshot returns a lock-safe copy of the window layout for paneID.
func (m *SessionManager) paneLayoutSnapshot(paneID int) (*LayoutNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pane, ok := m.panes[paneID]
	if !ok || pane == nil || pane.Window == nil || pane.Window.Session == nil {
		return nil, fmt.Errorf("pane not found: %%%d", paneID)
	}
	return cloneLayout(pane.Window.Layout), nil
}

// HasPane reports whether paneID (format "%N") is currently active.
func (m *SessionManager) HasPane(paneID string) bool {
	id, err := parsePaneID(strings.TrimSpace(paneID))
	if err != nil {
		return false
	}
	m.mu.RLock()
	_, ok := m.panes[id]
	m.mu.RUnlock()
	return ok
}
