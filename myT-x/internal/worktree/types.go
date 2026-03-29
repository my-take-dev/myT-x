package worktree

// WorktreeSessionOptions holds options for creating a session with a worktree.
//
// Mode semantics (invariant):
//   - BranchName is required (non-empty) -> Named branch mode
//   - BaseBranch == ""  -> Uses current HEAD as the base for the worktree
//
// NOTE: Detached HEAD mode was removed from the UI. Existing detached worktrees
// (via CreateSessionWithExistingWorktree) are still supported and can be
// promoted via PromoteWorktreeToBranch.
type WorktreeSessionOptions struct {
	BranchName          string `json:"branch_name"`            // required: branch name for the new worktree
	BaseBranch          string `json:"base_branch"`            // empty = current HEAD
	PullBeforeCreate    bool   `json:"pull_before_create"`     // pull latest before creating worktree
	EnableAgentTeam     bool   `json:"enable_agent_team"`      // set Agent Teams env vars on initial pane
	UseClaudeEnv        bool   `json:"use_claude_env"`         // apply claude_env config to panes
	UsePaneEnv          bool   `json:"use_pane_env"`           // apply pane_env config to additional panes
	UseSessionPaneScope bool   `json:"use_session_pane_scope"` // set MYTX_SESSION on panes + scope list-panes
}

// WorktreeStatus holds the pre-close status of a worktree session.
type WorktreeStatus struct {
	HasWorktree    bool   `json:"has_worktree"`
	HasUncommitted bool   `json:"has_uncommitted"`
	HasUnpushed    bool   `json:"has_unpushed"`
	BranchName     string `json:"branch_name"`
	IsDetached     bool   `json:"is_detached"`
}

// SessionEnvOptions holds environment configuration options for session creation.
// This mirrors the relevant fields from main.CreateSessionOptions to avoid
// circular package imports between main and internal/worktree.
type SessionEnvOptions struct {
	EnableAgentTeam     bool `json:"enable_agent_team"`      // set Agent Teams env vars on initial pane
	UseClaudeEnv        bool `json:"use_claude_env"`         // apply claude_env config to panes
	UsePaneEnv          bool `json:"use_pane_env"`           // apply pane_env config to additional panes
	UseSessionPaneScope bool `json:"use_session_pane_scope"` // set MYTX_SESSION on panes + scope list-panes
}

// copyWalkBudget tracks resource consumption during directory copy operations.
// Limits prevent accidental large recursive copies from blocking worktree
// creation or exhausting disk space.
type copyWalkBudget struct {
	fileCount int
	totalSize int64
}
