package main

import "myT-x/internal/inputhistory"

func (a *App) ensureInputHistoryService() *inputhistory.Service {
	a.inputHistoryServiceOnce.Do(func() {
		if a.inputHistoryService == nil {
			options := []inputhistory.Option{}
			if a.sessionService != nil {
				options = append(options, inputhistory.WithSessionScopeResolver(
					func(sessionName string) (string, error) {
						return a.sessionService.ResolveSessionWorkDir(sessionName)
					},
					appConfigDirProvider(a),
				))
			}
			a.inputHistoryService = inputhistory.NewService(
				newAppRuntimeEventEmitterAdapter(a),
				func() bool { return a.shuttingDown.Load() },
				options...,
			)
		}
	})
	return a.inputHistoryService
}

// initInputHistory creates the JSONL input history file for the current run.
// configPath is passed explicitly because this function is called before
// configState.Initialize() completes (to capture early startup warnings).
func (a *App) initInputHistory(configPath string) {
	a.ensureInputHistoryService().Init(configPath)
}

// cleanupOldInputHistory removes the oldest input history files when the count exceeds MaxFiles.
// test-only wrapper: production code calls Service.CleanupOldFiles() via Init().
func (a *App) cleanupOldInputHistory() {
	a.ensureInputHistoryService().CleanupOldFiles()
}

// processInputString strips terminal escape sequences from raw input.
// test-only wrapper: production code calls inputhistory.ProcessInputString() via RecordInput().
func processInputString(input string) string {
	return inputhistory.ProcessInputString(input)
}

// recordInput processes raw terminal input and appends complete command lines to history.
func (a *App) recordInput(paneID, input, source, session string) {
	a.ensureInputHistoryService().RecordInput(paneID, input, source, session)
}

// flushLineBuffer extracts and writes any pending buffered text for the given pane.
// test-only wrapper: production flush is triggered by internal timers or flushAllLineBuffers().
func (a *App) flushLineBuffer(paneID string, gen uint64) {
	a.ensureInputHistoryService().FlushLineBuffer(paneID, gen)
}

// flushAllLineBuffers stops all pending timers and writes any buffered text.
func (a *App) flushAllLineBuffers() {
	a.ensureInputHistoryService().FlushAllLineBuffers()
}

// writeInputHistoryEntry appends an entry to both the in-memory ring buffer and JSONL file.
// test-only wrapper: production code calls Service.WriteEntry() via RecordInput/FlushLineBuffer.
func (a *App) writeInputHistoryEntry(entry InputHistoryEntry) {
	a.ensureInputHistoryService().WriteEntry(entry)
}

// closeInputHistory flushes and closes the input history file handle.
func (a *App) closeInputHistory() {
	a.ensureInputHistoryService().Close()
}

// Deprecated: use GetInputHistoryForSession for scoped input history.
// GetInputHistory returns input history for the active session.
func (a *App) GetInputHistory() []InputHistoryEntry {
	activeSessionName := ""
	if a.sessionService != nil {
		activeSessionName = a.sessionService.GetActiveSessionName()
	}
	return a.ensureInputHistoryService().SnapshotForSession(activeSessionName).Entries
}

// GetInputHistoryForSession returns input history scoped to the session work directory.
func (a *App) GetInputHistoryForSession(sessionName string) inputhistory.Snapshot {
	return a.ensureInputHistoryService().SnapshotForSession(sessionName)
}

// GetInputHistoryFilePath returns the absolute path to the current session's JSONL input history file.
func (a *App) GetInputHistoryFilePath() string {
	activeSessionName := ""
	if a.sessionService != nil {
		activeSessionName = a.sessionService.GetActiveSessionName()
	}
	return a.ensureInputHistoryService().FilePathForSession(activeSessionName)
}
