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

// DevPanelReadBinary reads a file within a session's working directory as base64-encoded bytes.
// Wails-bound: called from file-view preview renderers.
func (a *App) DevPanelReadBinary(sessionName string, filePath string) (BinaryFileContent, error) {
	return a.devpanelService.ReadBinary(sessionName, filePath)
}

// DevPanelSqliteListTables returns browsable SQLite tables for a session-scoped database path.
// Wails-bound: called from the frontend developer panel file viewer.
func (a *App) DevPanelSqliteListTables(sessionName string, dbPath string) ([]SqliteTableInfo, error) {
	return a.devpanelService.SqliteListTables(sessionName, dbPath)
}

// DevPanelSqliteQueryTable returns a page of SQLite rows for a session-scoped database path.
// Wails-bound: called from the frontend developer panel file viewer.
func (a *App) DevPanelSqliteQueryTable(sessionName string, dbPath string, tableName string, offset int64, limit int) (SqliteQueryResult, error) {
	return a.devpanelService.SqliteQueryTable(sessionName, dbPath, tableName, offset, limit)
}

// DevPanelSqliteExportCSV exports a SQLite table/view to CSV under the session root.
// Wails-bound: called from the frontend developer panel file viewer.
func (a *App) DevPanelSqliteExportCSV(sessionName string, dbPath string, tableName string, destRelPath string) (SqliteExportResult, error) {
	return a.devpanelService.SqliteExportCSV(sessionName, dbPath, tableName, destRelPath)
}

// DevPanelGetFileInfo returns metadata for a file-system entry within a session's working directory.
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelGetFileInfo(sessionName string, filePath string) (FileMetadata, error) {
	return a.devpanelService.GetFileInfo(sessionName, filePath)
}

// DevPanelWriteFile writes content to a file within a session's working directory.
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelWriteFile(sessionKey string, filePath string, content string) (WriteFileResult, error) {
	sessionName, err := a.requireExistingSessionKey(sessionKey)
	if err != nil {
		return WriteFileResult{}, err
	}
	return a.devpanelService.WriteFile(sessionName, filePath, content)
}

// DevPanelCreateFile creates an empty file within a session's working directory.
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelCreateFile(sessionKey string, filePath string) (WriteFileResult, error) {
	sessionName, err := a.requireExistingSessionKey(sessionKey)
	if err != nil {
		return WriteFileResult{}, err
	}
	return a.devpanelService.CreateFile(sessionName, filePath)
}

// DevPanelCreateDirectory creates a directory within a session's working directory.
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelCreateDirectory(sessionKey string, dirPath string) error {
	sessionName, err := a.requireExistingSessionKey(sessionKey)
	if err != nil {
		return err
	}
	return a.devpanelService.CreateDirectory(sessionName, dirPath)
}

// DevPanelRenameFile renames or moves a file-system entry within a session's working directory.
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelRenameFile(sessionKey string, oldPath string, newPath string) error {
	sessionName, err := a.requireExistingSessionKey(sessionKey)
	if err != nil {
		return err
	}
	return a.devpanelService.RenameFile(sessionName, oldPath, newPath)
}

// DevPanelDeleteFile deletes a file-system entry within a session's working directory.
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelDeleteFile(sessionKey string, filePath string) error {
	sessionName, err := a.requireExistingSessionKey(sessionKey)
	if err != nil {
		return err
	}
	return a.devpanelService.DeleteFile(sessionName, filePath)
}

// DevPanelStartWatcher starts or references a filesystem watcher for a session.
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelStartWatcher(sessionKey string) error {
	sessionName, err := a.requireExistingSessionKey(sessionKey)
	if err != nil {
		return err
	}
	return a.devpanelService.StartWatcher(sessionName)
}

// DevPanelStopWatcher releases a filesystem watcher reference for a session.
// Wails-bound: called from the frontend developer panel.
func (a *App) DevPanelStopWatcher(sessionKey string) error {
	sessionName, err := a.requireExistingSessionKey(sessionKey)
	if err != nil {
		return err
	}
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
