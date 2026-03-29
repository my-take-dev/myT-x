package main

// ---------------------------------------------------------------------------
// Wails-bound thin wrappers for git operations — delegate to devpanelService
// ---------------------------------------------------------------------------

// DevPanelGitStage stages a file for commit (git add).
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelGitStage(sessionName string, path string) error {
	return a.devpanelService.GitStage(sessionName, path)
}

// DevPanelGitUnstage unstages a file (git restore --staged).
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelGitUnstage(sessionName string, path string) error {
	return a.devpanelService.GitUnstage(sessionName, path)
}

// DevPanelGitDiscard discards working changes for a file.
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelGitDiscard(sessionName string, path string) error {
	return a.devpanelService.GitDiscard(sessionName, path)
}

// DevPanelGitStageAll stages all changes for commit (git add -A).
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelGitStageAll(sessionName string) error {
	return a.devpanelService.GitStageAll(sessionName)
}

// DevPanelGitUnstageAll unstages all staged changes.
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelGitUnstageAll(sessionName string) error {
	return a.devpanelService.GitUnstageAll(sessionName)
}

// DevPanelGitCommit creates a commit with the currently staged changes.
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelGitCommit(sessionName string, message string) (DevPanelCommitResult, error) {
	return a.devpanelService.GitCommit(sessionName, message)
}

// DevPanelGitPush pushes the current branch to its remote.
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelGitPush(sessionName string) (DevPanelPushResult, error) {
	return a.devpanelService.GitPush(sessionName)
}

// DevPanelGitPull pulls changes from the remote for the current branch.
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelGitPull(sessionName string) (DevPanelPullResult, error) {
	return a.devpanelService.GitPull(sessionName)
}

// DevPanelGitFetch fetches from all remotes and prunes deleted references.
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelGitFetch(sessionName string) error {
	return a.devpanelService.GitFetch(sessionName)
}
