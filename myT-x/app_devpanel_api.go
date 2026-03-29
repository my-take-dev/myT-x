package main

// ---------------------------------------------------------------------------
// Wails-bound thin wrappers — delegate to devpanelService
// ---------------------------------------------------------------------------

// DevPanelListDir returns the contents of a directory within a session's working directory.
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelListDir(sessionName string, dirPath string) ([]FileEntry, error) {
	return a.devpanelService.ListDir(sessionName, dirPath)
}

// DevPanelReadFile reads a file within a session's working directory.
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelReadFile(sessionName string, filePath string) (FileContent, error) {
	return a.devpanelService.ReadFile(sessionName, filePath)
}

// DevPanelGitLog returns the commit history for a session's repository.
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelGitLog(sessionName string, maxCount int, allBranches bool) ([]GitGraphCommit, error) {
	return a.devpanelService.GitLog(sessionName, maxCount, allBranches)
}

// DevPanelGitStatus returns the working tree status for a session's repository.
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelGitStatus(sessionName string) (GitStatusResult, error) {
	return a.devpanelService.GitStatus(sessionName)
}

// DevPanelCommitDiff returns the unified diff for a specific commit.
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelCommitDiff(sessionName string, commitHash string) (string, error) {
	return a.devpanelService.CommitDiff(sessionName, commitHash)
}

// DevPanelWorkingDiff returns the unified diff of working changes (staged + unstaged) vs HEAD,
// plus synthetic diffs for untracked (new) files.
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelWorkingDiff(sessionName string) (WorkingDiffResult, error) {
	return a.devpanelService.WorkingDiff(sessionName)
}

// DevPanelListBranches returns all branch names for a session's repository.
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelListBranches(sessionName string) ([]string, error) {
	return a.devpanelService.ListBranches(sessionName)
}

// DevPanelSearchFiles searches for files by name and content within a session's working directory.
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelSearchFiles(sessionName string, query string) ([]SearchFileResult, error) {
	return a.devpanelService.SearchFiles(sessionName, query)
}
