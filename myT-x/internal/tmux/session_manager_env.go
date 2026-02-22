package tmux

import (
	"fmt"
	"strings"

	"myT-x/internal/terminal"
)

// INVARIANT (copy-on-read contract for env maps): Every public method in this
// file that returns a map[string]string (GetSessionEnv, GetPaneEnv, etc.)
// returns a deep copy via copyEnvMap. Callers may freely mutate the returned
// map without affecting internal state. This is the complementary read-side
// contract to the copy-on-write contract documented on paneEnvView in
// command_router.go. Both contracts together ensure that env maps are never
// shared across goroutine boundaries without protection.
//
// The write-side methods (SetSessionEnv, SetPaneRuntime, etc.) always mutate
// the canonical session/pane Env map under m.mu.Lock, so no copy-on-write
// indirection is needed here — the mutex serialises all writes.
//
// NOTE: SetPaneRuntime also applies copy-on-write for the incoming env map:
// it calls copyEnvMap(env) before storing, ensuring the caller's map is not
// aliased into internal state. This completes the bidirectional isolation
// contract for pane environment maps.

// PaneContextSnapshot is a lock-safe snapshot of pane-owned session/window state.
type PaneContextSnapshot struct {
	SessionID   int
	SessionName string
	WindowID    int
	Layout      *LayoutNode
	Env         map[string]string
	Title       string
	// SessionWorkDir is the effective working directory for the session.
	// Worktree sessions use Worktree.Path; regular sessions use RootPath.
	SessionWorkDir string
	// PaneWidth and PaneHeight are the pane's column/row dimensions at snapshot
	// time. Added for I-02 so that handleResizePane can read fallback dimensions
	// under RLock instead of dereferencing the live pointer after lock release.
	PaneWidth  int
	PaneHeight int
}

// parseSessionName extracts the session name from a target string.
// If name contains a colon, only the portion before it is used
// (e.g., "mysession:0" -> "mysession").
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

// SetSessionEnv sets a single environment variable on the named session.
func (m *SessionManager) SetSessionEnv(name, key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("environment variable name is required")
	}

	session, err := m.getSessionByNameLocked(name)
	if err != nil {
		return err
	}
	if session.Env == nil {
		session.Env = map[string]string{}
	}
	if prev, exists := session.Env[key]; exists && prev == value {
		return nil
	}
	session.Env[key] = value
	m.markStateMutationLocked()
	return nil
}

// UnsetSessionEnv removes a single environment variable from the named session.
func (m *SessionManager) UnsetSessionEnv(name, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("environment variable name is required")
	}

	session, err := m.getSessionByNameLocked(name)
	if err != nil {
		return err
	}
	if session.Env != nil {
		if _, exists := session.Env[key]; !exists {
			return nil
		}
		delete(session.Env, key)
		m.markStateMutationLocked()
	}
	return nil
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
		if session.Worktree != nil {
			m.markStateMutationLocked()
		}
		session.Worktree = nil
		return nil
	}

	if !worktreeInfoEqual(session.Worktree, normalized) {
		m.markStateMutationLocked()
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
	rootPath = strings.TrimSpace(rootPath)
	session, err := m.getSessionByNameLocked(name)
	if err != nil {
		return err
	}
	if session.RootPath != rootPath {
		m.markStateMutationLocked()
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
	if session.IsAgentTeam != isAgent {
		m.markStateMutationLocked()
	}
	session.IsAgentTeam = isAgent
	return nil
}

// SetUseClaudeEnv sets whether claude_env is applied to panes in the named session.
func (m *SessionManager) SetUseClaudeEnv(name string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, err := m.getSessionByNameLocked(name)
	if err != nil {
		return err
	}
	if session.UseClaudeEnv == nil || *session.UseClaudeEnv != enabled {
		m.markStateMutationLocked()
	}
	session.UseClaudeEnv = &enabled
	return nil
}

// SetUsePaneEnv sets whether pane_env is applied to additional panes in the named session.
func (m *SessionManager) SetUsePaneEnv(name string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, err := m.getSessionByNameLocked(name)
	if err != nil {
		return err
	}
	if session.UsePaneEnv == nil || *session.UsePaneEnv != enabled {
		m.markStateMutationLocked()
	}
	session.UsePaneEnv = &enabled
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
		m.markStateMutationLocked()
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

	// Resolve effective working directory.
	sess := pane.Window.Session
	workDir := strings.TrimSpace(sess.RootPath)
	if wt := sess.Worktree; wt != nil && strings.TrimSpace(wt.Path) != "" {
		workDir = strings.TrimSpace(wt.Path)
	}

	return PaneContextSnapshot{
		SessionID:      sess.ID,
		SessionName:    sess.Name,
		WindowID:       pane.Window.ID,
		Layout:         cloneLayout(pane.Window.Layout),
		Env:            copyEnvMap(pane.Env),
		Title:          pane.Title,
		SessionWorkDir: workDir,
		PaneWidth:      pane.Width,
		PaneHeight:     pane.Height,
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

func worktreeInfoEqual(left, right *SessionWorktreeInfo) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.Path == right.Path &&
		left.RepoPath == right.RepoPath &&
		left.BranchName == right.BranchName &&
		left.BaseBranch == right.BaseBranch &&
		left.IsDetached == right.IsDetached
}
