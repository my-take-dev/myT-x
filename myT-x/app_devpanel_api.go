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

// DevPanelGetFileInfo returns metadata for a file-system entry within a session's working directory.
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelGetFileInfo(sessionName string, filePath string) (FileMetadata, error) {
	return a.devpanelService.GetFileInfo(sessionName, filePath)
}

// DevPanelWriteFile writes content to a file within a session's working directory.
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelWriteFile(sessionName string, filePath string, content string) (WriteFileResult, error) {
	return a.devpanelService.WriteFile(sessionName, filePath, content)
}

// DevPanelCreateFile creates an empty file within a session's working directory.
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelCreateFile(sessionName string, filePath string) (WriteFileResult, error) {
	return a.devpanelService.CreateFile(sessionName, filePath)
}

// DevPanelCreateDirectory creates a directory within a session's working directory.
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelCreateDirectory(sessionName string, dirPath string) error {
	return a.devpanelService.CreateDirectory(sessionName, dirPath)
}

// DevPanelRenameFile renames or moves a file-system entry within a session's working directory.
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelRenameFile(sessionName string, oldPath string, newPath string) error {
	return a.devpanelService.RenameFile(sessionName, oldPath, newPath)
}

// DevPanelDeleteFile deletes a file-system entry within a session's working directory.
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelDeleteFile(sessionName string, filePath string) error {
	return a.devpanelService.DeleteFile(sessionName, filePath)
}

// DevPanelStartWatcher starts or references a filesystem watcher for a session.
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelStartWatcher(sessionName string) error {
	return a.devpanelService.StartWatcher(sessionName)
}

// DevPanelStopWatcher releases a filesystem watcher reference for a session.
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelStopWatcher(sessionName string) error {
	return a.devpanelService.StopWatcher(sessionName)
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
