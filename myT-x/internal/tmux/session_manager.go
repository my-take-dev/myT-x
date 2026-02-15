package tmux

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"myT-x/internal/terminal"
)

// TmuxSession models a tmux-like session.
type TmuxSession struct {
	ID           int               `json:"id"`
	Name         string            `json:"name"`
	Windows      []*TmuxWindow     `json:"windows"`
	CreatedAt    time.Time         `json:"created_at"`
	LastActivity time.Time         `json:"-"`
	IsIdle       bool              `json:"-"`
	Env          map[string]string `json:"env,omitempty"`

	// IsAgentTeam is omitted when false. Frontend treats missing as false.
	IsAgentTeam bool `json:"is_agent_team,omitempty"`

	// Worktree metadata grouped as one logical unit.
	// Nil means no worktree-related metadata is attached to the session.
	Worktree *SessionWorktreeInfo `json:"worktree,omitempty"`

	// RootPath is the user-selected directory at session creation time.
	// Set via SetRootPath independently from SetWorktreeInfo; for worktree sessions,
	// this stores the repository root (not the worktree directory) for conflict detection.
	RootPath string `json:"root_path,omitempty"`
}

// SessionWorktreeInfo is frontend-safe git/worktree metadata for a session.
//
// Variant semantics:
//   - Worktree session: Path is non-empty (worktree directory), RepoPath is repository root.
//   - Repo-tracked session: Path is empty, RepoPath/BranchName carry repository metadata only.
type SessionWorktreeInfo struct {
	Path       string `json:"path,omitempty"`
	RepoPath   string `json:"repo_path,omitempty"`
	BranchName string `json:"branch_name,omitempty"`
	BaseBranch string `json:"base_branch,omitempty"`
	// Keep explicit false in JSON so frontend can distinguish false from missing.
	IsDetached bool `json:"is_detached"`
}

// IsEmpty reports whether worktree metadata carries no meaningful value.
// This function assumes fields are already normalized; callers that accept
// user input must trim whitespace before calling IsEmpty.
func (info *SessionWorktreeInfo) IsEmpty() bool {
	if info == nil {
		return true
	}
	return info.Path == "" &&
		info.RepoPath == "" &&
		info.BranchName == "" &&
		info.BaseBranch == "" &&
		!info.IsDetached
}

// IsWorktreeSession reports whether this metadata points to an actual worktree path.
func (info *SessionWorktreeInfo) IsWorktreeSession() bool {
	return info != nil && strings.TrimSpace(info.Path) != ""
}

// TmuxWindow models a tmux-like window.
type TmuxWindow struct {
	ID     int         `json:"id"`
	Name   string      `json:"name"`
	Panes  []*TmuxPane `json:"panes"`
	Layout *LayoutNode `json:"layout"`
	// ActivePN is the 0-based index into the Panes slice identifying the active pane.
	// Kept in sync with TmuxPane.Index (which equals the pane's slice position).
	ActivePN int          `json:"active_pane"`
	Session  *TmuxSession `json:"-"`
}

// TmuxPane models a tmux-like pane.
type TmuxPane struct {
	ID       int                `json:"id"`
	Index    int                `json:"index"`
	Terminal *terminal.Terminal `json:"-"`
	Title    string             `json:"title,omitempty"`
	Active   bool               `json:"active"`
	Width    int                `json:"width"`
	Height   int                `json:"height"`
	Env      map[string]string  `json:"env,omitempty"`
	Window   *TmuxWindow        `json:"-"`
}

func (p *TmuxPane) IDString() string {
	return fmt.Sprintf("%%%d", p.ID)
}

// PaneSnapshot is a frontend-safe pane representation.
type PaneSnapshot struct {
	ID     string `json:"id"`
	Index  int    `json:"index"`
	Title  string `json:"title,omitempty"`
	Active bool   `json:"active"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// WindowSnapshot is a frontend-safe window representation.
type WindowSnapshot struct {
	ID     int         `json:"id"`
	Name   string      `json:"name"`
	Layout *LayoutNode `json:"layout"`
	// ActivePN is the 0-based index into the Panes slice identifying the active pane.
	// Mirrors TmuxWindow.ActivePN.
	ActivePN int            `json:"active_pane"`
	Panes    []PaneSnapshot `json:"panes"`
}

// SessionSnapshot is a frontend-safe session representation.
type SessionSnapshot struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	IsIdle    bool      `json:"is_idle"`
	// IsAgentTeam is omitted when false. Frontend treats missing as false.
	IsAgentTeam bool             `json:"is_agent_team,omitempty"`
	Windows     []WindowSnapshot `json:"windows"`

	Worktree *SessionWorktreeInfo `json:"worktree,omitempty"`
	RootPath string               `json:"root_path,omitempty"`
}

func (p *TmuxPane) ttyPath() string {
	return fmt.Sprintf(`\\.\conpty\%%%d`, p.ID)
}

// SessionManager owns session/window/pane state.
type SessionManager struct {
	sessions map[string]*TmuxSession
	panes    map[int]*TmuxPane

	nextSessionID int
	nextPaneID    int
	now           func() time.Time
	idleThreshold time.Duration

	mu sync.RWMutex
}

// NewSessionManager creates a SessionManager.
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions:      map[string]*TmuxSession{},
		panes:         map[int]*TmuxPane{},
		now:           time.Now,
		idleThreshold: 5 * time.Second,
	}
}
