package session

// CreateSessionOptions holds the boolean options for session creation.
// The main package defines its own CreateSessionOptions with JSON tags for
// Wails binding; the App layer maps between the two types.
type CreateSessionOptions struct {
	EnableAgentTeam     bool // set Agent Teams env vars on initial pane
	UseClaudeEnv        bool // apply claude_env config to panes
	UsePaneEnv          bool // apply pane_env config to additional panes
	UseSessionPaneScope bool // set MYTX_SESSION on panes + scope list-panes
}

// WorktreeCleanupParams holds parameters for CleanupSessionWorktree.
type WorktreeCleanupParams struct {
	SessionName string
	WtPath      string
	RepoPath    string
	BranchName  string
}
