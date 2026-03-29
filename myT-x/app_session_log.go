package main

import "myT-x/internal/sessionlog"

func (a *App) ensureSessionLogService() *sessionlog.Service {
	a.sessionLogServiceOnce.Do(func() {
		if a.sessionLogService == nil {
			a.sessionLogService = sessionlog.NewService(
				newAppRuntimeEventEmitterAdapter(a),
				func() bool { return a.shuttingDown.Load() },
			)
		}
	})
	return a.sessionLogService
}

// initSessionLog creates the JSONL session log file for the current run.
// configPath is passed explicitly because this function is called before
// configState.Initialize() completes (to capture early startup warnings).
func (a *App) initSessionLog(configPath string) {
	a.ensureSessionLogService().Init(configPath)
}

// writeSessionLogEntry appends an entry to both the in-memory ring buffer and
// the JSONL file.
func (a *App) writeSessionLogEntry(entry SessionLogEntry) {
	a.ensureSessionLogService().WriteEntry(entry)
}

// closeSessionLog flushes and closes the session log file handle.
func (a *App) closeSessionLog() {
	a.ensureSessionLogService().Close()
}

// GetSessionErrorLog returns a copy of all in-memory session log entries.
// Wails-bound: called from the frontend to display the error log panel.
func (a *App) GetSessionErrorLog() []SessionLogEntry {
	return a.ensureSessionLogService().Snapshot()
}

// GetSessionLogFilePath returns the absolute path to the current session's JSONL log file.
// Wails-bound: used by the frontend for "open log file" actions.
func (a *App) GetSessionLogFilePath() string {
	return a.ensureSessionLogService().FilePath()
}

// LogFrontendEvent writes a frontend-originated event into the session log.
// Wails-bound: called by the frontend to persist UI-layer errors.
func (a *App) LogFrontendEvent(level, msg, source string) {
	a.ensureSessionLogService().LogFrontendEvent(level, msg, source)
}
