package tmux

import (
	"fmt"
	"strings"
	"time"

	"myT-x/internal/terminal"
)

// ---------------------------------------------------------------------------
// Session / Window / Pane model types
// ---------------------------------------------------------------------------

// TmuxSession models a tmux-like session.
type TmuxSession struct {
	ID             int               `json:"id"`
	Name           string            `json:"name"`
	Windows        []*TmuxWindow     `json:"windows"`
	ActiveWindowID int               `json:"active_window_id"`
	CreatedAt      time.Time         `json:"created_at"`
	LastActivity   time.Time         `json:"-"`
	IsIdle         bool              `json:"-"`
	Env            map[string]string `json:"env,omitempty"`

	// IsAgentTeam is omitted when false. Frontend treats missing as false.
	IsAgentTeam bool `json:"is_agent_team,omitempty"`

	// Worktree metadata grouped as one logical unit.
	// Nil means no worktree-related metadata is attached to the session.
	Worktree *SessionWorktreeInfo `json:"worktree,omitempty"`

	// RootPath is the user-selected directory at session creation time.
	// Set via SetRootPath independently from SetWorktreeInfo; for worktree sessions,
	// this stores the repository root (not the worktree directory) for conflict detection.
	RootPath string `json:"root_path,omitempty"`

	// UseClaudeEnv controls whether claude_env config is applied to panes in this session.
	// nil = legacy session (IPC-created), pointer semantics distinguish unset from false.
	UseClaudeEnv *bool `json:"use_claude_env,omitempty"`
	// UsePaneEnv controls whether pane_env config is applied to additional panes.
	// nil = legacy session (pane_env always fills, backward compatible).
	UsePaneEnv *bool `json:"use_pane_env,omitempty"`
	// UseSessionPaneScope controls whether MYTX_SESSION is set on additional panes
	// and list-panes -a is scoped to the caller's session.
	// nil = legacy session (no session scoping, backward compatible).
	UseSessionPaneScope *bool `json:"use_session_pane_scope,omitempty"`
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
	ID            int                `json:"id"`
	idString      string             `json:"-"`
	Index         int                `json:"index"`
	Terminal      *terminal.Terminal `json:"-"`
	Title         string             `json:"title,omitempty"`
	Active        bool               `json:"active"`
	Width         int                `json:"width"`
	Height        int                `json:"height"`
	Env           map[string]string  `json:"env,omitempty"`
	OutputHistory *PaneOutputHistory `json:"-"`
	Window        *TmuxWindow        `json:"-"`
}

// IDString returns the pane identifier in tmux "%N" format.
func (p *TmuxPane) IDString() string {
	if p == nil {
		return ""
	}
	if p.idString != "" {
		return p.idString
	}
	return fmt.Sprintf("%%%d", p.ID)
}

func (p *TmuxPane) ttyPath() string {
	return fmt.Sprintf(`\\.\conpty\%%%d`, p.ID)
}

// ---------------------------------------------------------------------------
// Snapshot types (frontend-safe, read-only representations)
// ---------------------------------------------------------------------------

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
	// ActiveWindowID identifies the active window in this session snapshot.
	ActiveWindowID int `json:"active_window_id"`
	// IsAgentTeam is omitted when false. Frontend treats missing as false.
	IsAgentTeam bool             `json:"is_agent_team,omitempty"`
	Windows     []WindowSnapshot `json:"windows"`

	Worktree *SessionWorktreeInfo `json:"worktree,omitempty"`
	RootPath string               `json:"root_path,omitempty"`
}

// Clone returns a deep copy of the SessionSnapshot.
func (ss SessionSnapshot) Clone() SessionSnapshot {
	out := ss

	if ss.Worktree != nil {
		worktreeCopy := *ss.Worktree
		out.Worktree = &worktreeCopy
	}

	if len(ss.Windows) == 0 {
		out.Windows = []WindowSnapshot{}
		return out
	}

	out.Windows = make([]WindowSnapshot, len(ss.Windows))
	for j := range ss.Windows {
		window := ss.Windows[j]
		out.Windows[j] = window
		out.Windows[j].Layout = cloneLayout(window.Layout)

		if len(window.Panes) == 0 {
			out.Windows[j].Panes = []PaneSnapshot{}
			continue
		}
		out.Windows[j].Panes = make([]PaneSnapshot, len(window.Panes))
		copy(out.Windows[j].Panes, window.Panes)
	}
	return out
}

// SessionSnapshotDelta represents incremental updates for session snapshots.
type SessionSnapshotDelta struct {
	Upserts []SessionSnapshot `json:"upserts"`
	// Removed contains the names (not IDs) of sessions that were removed since the
	// previous snapshot. Frontend consumers should match these against session.name
	// fields rather than session.id.
	Removed []string `json:"removed"`
}

// ---------------------------------------------------------------------------
// Event / context snapshot types
// ---------------------------------------------------------------------------

// PaneOutputEvent carries one terminal output chunk for frontend delivery.
type PaneOutputEvent struct {
	PaneID string
	Data   []byte
}

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

// PanePIDInfo はペインIDとシェルプロセスPIDの組を表す。
type PanePIDInfo struct {
	PaneID string
	PID    int
}
