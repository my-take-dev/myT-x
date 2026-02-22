package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"
)

const (
	sessionLogDir             = "session-logs"
	sessionLogMaxFiles        = 100
	sessionLogMaxEntries      = 10000
	sessionLogEmitMinInterval = 50 * time.Millisecond
)

// initSessionLog creates the JSONL session log file for the current run.
// Non-fatal: logs a warning and continues if any I/O operation fails.
func (a *App) initSessionLog() {
	dir := filepath.Join(filepath.Dir(a.configPath), sessionLogDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		slog.Warn("[session-log] failed to create log directory", "dir", dir, "error", err)
		return
	}

	// NOTE(A-2): PID is appended to prevent filename collision on sub-second restart.
	filename := fmt.Sprintf("session-%s-%d.jsonl", time.Now().Format("20060102-150405"), os.Getpid())
	fullPath := filepath.Join(dir, filename)

	f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		slog.Warn("[session-log] failed to open log file", "path", fullPath, "error", err)
		return
	}

	// Write shared fields under lock so concurrent readers (GetSessionLogFilePath,
	// writeSessionLogEntry) always observe a consistent pair.
	a.sessionLogMu.Lock()
	a.sessionLogFile = f
	a.sessionLogPath = fullPath
	a.sessionLogMu.Unlock()

	a.cleanupOldSessionLogs()

	slog.Info("[session-log] initialized", "path", fullPath)
}

// cleanupOldSessionLogs removes the oldest session log files when the count
// exceeds sessionLogMaxFiles.
func (a *App) cleanupOldSessionLogs() {
	a.sessionLogMu.RLock()
	currentPath := a.sessionLogPath
	a.sessionLogMu.RUnlock()
	if strings.TrimSpace(currentPath) == "" {
		return
	}

	logDir := filepath.Dir(currentPath)
	currentFile := filepath.Base(currentPath)
	entries, err := os.ReadDir(logDir)
	if err != nil {
		slog.Warn("[session-log] failed to read log directory for cleanup", "dir", logDir, "error", err)
		return
	}

	var logFiles []string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && strings.HasPrefix(name, "session-") && strings.HasSuffix(name, ".jsonl") {
			logFiles = append(logFiles, name)
		}
	}

	// NOTE: sort.Strings sorts lexicographically. Files with the same timestamp but
	// different PIDs are ordered by PID string value (not numeric). This is acceptable
	// because cleanup only needs approximate age ordering -- the timestamp prefix
	// ensures files are primarily ordered by creation time.
	sort.Strings(logFiles)

	excess := len(logFiles) - sessionLogMaxFiles
	if excess <= 0 {
		return
	}

	deleted := 0
	for _, name := range logFiles {
		if deleted >= excess {
			break
		}
		if name == currentFile {
			// Never delete the active session log file for this process.
			continue
		}
		// NOTE: Cleanup only protects the current process's active file. In rare
		// multi-process runs sharing the same config directory, another process may
		// still be writing to an older file while this process trims by count.
		// This is an accepted limitation of the local best-effort cleanup policy.
		target := filepath.Join(logDir, name)
		if err := os.Remove(target); err != nil {
			slog.Warn("[session-log] failed to delete old log file", "path", target, "error", err)
			continue
		}
		slog.Debug("[session-log] deleted old log file", "path", target)
		deleted++
	}
}

// writeSessionLogEntry appends an entry to both the in-memory ring buffer and
// the JSONL file. All state mutations and the shouldEmit decision are made in a
// single lock acquisition to eliminate the race window between the two former
// critical sections.
//
// NOTE(A-0): The event model uses "ping + fetch" -- the emitted event
// "app:session-log-updated" carries no payload. The frontend receives the ping
// and calls GetSessionErrorLog() to obtain the full snapshot. This eliminates
// data loss from throttling: even if pings are throttled, the next ping triggers
// a full snapshot fetch that includes all entries.
//
// NOTE(A-0b): Sync() is called outside the mutex to prevent disk I/O latency.
// The file pointer is captured under lock and synced after unlock.
func (a *App) writeSessionLogEntry(entry SessionLogEntry) {
	// NOTE: slog.Warn/Error must NOT be called while sessionLogMu is held.
	// The TeeHandler intercepts slog records and calls this function back,
	// which would deadlock on the non-reentrant mutex. Internal diagnostics
	// use fmt.Fprintf(os.Stderr, ...) to bypass the TeeHandler entirely.

	var marshalErr, writeErr error
	shouldEmit := false
	var syncFile *os.File

	a.sessionLogMu.Lock()

	// --- assign monotonic sequence number ---
	a.sessionLogSeq++
	entry.Seq = a.sessionLogSeq

	// --- file persistence ---
	if a.sessionLogFile != nil {
		raw, err := json.Marshal(entry)
		if err != nil {
			marshalErr = err
		} else {
			raw = append(raw, '\n')
			if _, err := a.sessionLogFile.Write(raw); err != nil {
				writeErr = err
			} else if entry.Level == "error" {
				// Capture while holding the lock, then sync after unlock.
				syncFile = a.sessionLogFile
			}
		}
	}

	// --- in-memory ring buffer append ---
	a.sessionLogEntries.push(entry)

	// --- throttle decision (single lock acquisition) ---
	now := time.Now()
	if now.Sub(a.sessionLogLastEmit) >= sessionLogEmitMinInterval {
		a.sessionLogLastEmit = now
		shouldEmit = true
	}

	a.sessionLogMu.Unlock()

	// --- post-lock I/O: Sync() for error-level entries (A-0b) ---
	if syncFile != nil {
		if syncErr := syncFile.Sync(); syncErr != nil {
			// NOTE: os.ErrClosed may occur during shutdown if closeSessionLog()
			// races with this post-lock Sync(). This is benign -- the file was
			// already flushed and closed by closeSessionLog().
			// Windows can also return EINVAL when Sync races with file closure.
			isExpectedCloseRace := errors.Is(syncErr, os.ErrClosed) ||
				(runtime.GOOS == "windows" && errors.Is(syncErr, syscall.EINVAL))
			if !isExpectedCloseRace {
				fmt.Fprintf(os.Stderr, "[session-log] failed to sync log file: %v\n", syncErr)
			}
		}
	}

	// Log internal errors via stderr to avoid TeeHandler -> writeSessionLogEntry recursion.
	if marshalErr != nil {
		fmt.Fprintf(os.Stderr, "[session-log] failed to marshal log entry: %v\n", marshalErr)
	}
	if writeErr != nil {
		fmt.Fprintf(os.Stderr, "[session-log] failed to write log entry: %v\n", writeErr)
	}

	// NOTE(A-0): Emit lightweight ping outside lock. The frontend calls
	// GetSessionErrorLog() to fetch the full snapshot on receipt.
	// Throttling only affects the ping frequency, never causes data loss.
	//
	// NOTE: nil is passed as payload, which arrives as JSON null on the frontend.
	// This is intentional -- the event is a notification trigger, not a data carrier.
	if shouldEmit {
		a.emitRuntimeEvent("app:session-log-updated", nil)
	}
}

// closeSessionLog flushes and closes the session log file handle.
func (a *App) closeSessionLog() {
	var closeErr error

	a.sessionLogMu.Lock()
	if a.sessionLogFile != nil {
		closeErr = a.sessionLogFile.Close()
		a.sessionLogFile = nil
	}
	a.sessionLogMu.Unlock()

	if closeErr != nil {
		fmt.Fprintf(os.Stderr, "[session-log] failed to close log file: %v\n", closeErr)
	}
}

// GetSessionErrorLog returns a copy of all in-memory session log entries.
// Wails-bound: called from the frontend to display the error log panel.
// The frontend calls this after receiving an "app:session-log-updated" ping event,
// ensuring it always has the complete snapshot regardless of ping throttling.
// Uses RLock because this is a read-only operation on the ring buffer (#53).
func (a *App) GetSessionErrorLog() []SessionLogEntry {
	a.sessionLogMu.RLock()
	defer a.sessionLogMu.RUnlock()

	return a.sessionLogEntries.snapshot()
}

// GetSessionLogFilePath returns the absolute path to the current session's JSONL log file.
// Wails-bound: used by the frontend for "open log file" actions.
// Uses RLock because this is a read-only operation (#53).
func (a *App) GetSessionLogFilePath() string {
	a.sessionLogMu.RLock()
	defer a.sessionLogMu.RUnlock()
	return a.sessionLogPath
}
