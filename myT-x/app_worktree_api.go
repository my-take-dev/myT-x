package main

import (
	gitpkg "myT-x/internal/git"
	"myT-x/internal/tmux"
	"myT-x/internal/worktree"
)

// ---------------------------------------------------------------------------
// Wails-bound thin wrappers — delegate to worktreeService
// ---------------------------------------------------------------------------

// CreateSessionWithWorktree creates a new session backed by a git worktree.
// Wails-bound: called from the frontend.
func (a *App) CreateSessionWithWorktree(
	repoPath string,
	sessionName string,
	opts WorktreeSessionOptions,
) (tmux.SessionSnapshot, error) {
	return a.worktreeService.CreateSessionWithWorktree(repoPath, sessionName, opts)
}

// CreateSessionWithExistingWorktree creates a session using an existing worktree.
// Wails-bound: called from the frontend.
func (a *App) CreateSessionWithExistingWorktree(
	repoPath string,
	sessionName string,
	worktreePath string,
	opts CreateSessionOptions,
) (tmux.SessionSnapshot, error) {
	return a.worktreeService.CreateSessionWithExistingWorktree(repoPath, sessionName, worktreePath, worktree.SessionEnvOptions{
		EnableAgentTeam:     opts.EnableAgentTeam,
		UseClaudeEnv:        opts.UseClaudeEnv,
		UsePaneEnv:          opts.UsePaneEnv,
		UseSessionPaneScope: opts.UseSessionPaneScope,
	})
}

// CleanupWorktree manually removes the worktree associated with a session.
// Wails-bound: called from the frontend.
func (a *App) CleanupWorktree(sessionName string) error {
	return a.worktreeService.CleanupWorktree(sessionName)
}

// CheckWorktreeStatus returns the worktree status for a session.
// Wails-bound: called from the frontend.
func (a *App) CheckWorktreeStatus(sessionName string) (WorktreeStatus, error) {
	return a.worktreeService.CheckWorktreeStatus(sessionName)
}

// CommitAndPushWorktree commits and/or pushes changes in the session's worktree.
// Wails-bound: called from the frontend.
func (a *App) CommitAndPushWorktree(sessionName, commitMessage string, push bool) error {
	return a.worktreeService.CommitAndPushWorktree(sessionName, commitMessage, push)
}

// PromoteWorktreeToBranch promotes a detached HEAD worktree to a named branch.
// Wails-bound: called from the frontend.
func (a *App) PromoteWorktreeToBranch(sessionName string, branchName string) error {
	return a.worktreeService.PromoteWorktreeToBranch(sessionName, branchName)
}

// ListWorktreesByRepo returns all worktree information for a given repository.
// Wails-bound: called from the frontend.
func (a *App) ListWorktreesByRepo(repoPath string) ([]gitpkg.WorktreeInfo, error) {
	return a.worktreeService.ListWorktreesByRepo(repoPath)
}

// ListBranches returns all local branch names for the repository at the given path.
// Wails-bound: called from the frontend.
func (a *App) ListBranches(repoPath string) ([]string, error) {
	return a.worktreeService.ListBranches(repoPath)
}

// GetCurrentBranch returns the current branch of the repository at repoPath.
// Wails-bound: called from the frontend.
func (a *App) GetCurrentBranch(repoPath string) (string, error) {
	return a.worktreeService.GetCurrentBranch(repoPath)
}

// IsGitRepository checks if the given path is a git repository.
// Wails-bound: called from the frontend.
func (a *App) IsGitRepository(path string) bool {
	return a.worktreeService.IsGitRepository(path)
}

// CheckWorktreePathConflict checks whether the given worktree path is already
// used by an active session. Returns the session name if conflict exists, or "".
// Wails-bound: called from the frontend.
func (a *App) CheckWorktreePathConflict(worktreePath string) string {
	return a.worktreeService.CheckWorktreePathConflict(worktreePath)
}

// ListOrphanedWorktrees returns worktree directories not associated with any
// active session. These are candidates for manual cleanup.
// Wails-bound: called from the frontend.
func (a *App) ListOrphanedWorktrees(repoPath string) ([]worktree.OrphanedWorktree, error) {
	return a.worktreeService.ListOrphanedWorktrees(repoPath)
}
