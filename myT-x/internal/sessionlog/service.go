package sessionlog

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"myT-x/internal/apptypes"
)

// Service manages session log persistence, ring buffer, and event emission.
type Service struct {
	emitter        apptypes.RuntimeEventEmitter
	isShuttingDown func() bool
	mu             sync.RWMutex
	file           *os.File
	path           string
	entries        ringBuffer
	lastEmit       time.Time
	seq            uint64
}

// NewService creates a new session log service.
func NewService(emitter apptypes.RuntimeEventEmitter, isShuttingDown func() bool) *Service {
	if isShuttingDown == nil {
		isShuttingDown = func() bool { return false }
	}
	return &Service{
		emitter:        emitter,
		isShuttingDown: isShuttingDown,
		entries:        newRingBuffer(MaxEntries),
	}
}

// Init creates the JSONL session log file for the current run.
// Non-fatal: logs a warning and continues if any I/O operation fails.
//
// NOTE: slog.Warn/Error must NOT be called while s.mu is held.
// The TeeHandler intercepts slog records and calls WriteEntry, which would
// deadlock on the non-reentrant mutex. Current slog calls are placed outside
// lock scope intentionally.
func (s *Service) Init(configPath string) {
	dir := filepath.Join(filepath.Dir(configPath), Dir)
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

	// Write shared fields under lock so concurrent readers (Snapshot,
	// WriteEntry) always observe a consistent pair.
	s.mu.Lock()
	s.file = f
	s.path = fullPath
	s.mu.Unlock()

	s.CleanupOldFiles()

	slog.Info("[session-log] initialized", "path", fullPath)
}

// parseFileSortKey extracts the timestamp and PID from a session log filename.
func parseFileSortKey(name string) (timestamp string, pid int, ok bool) {
	if !strings.HasPrefix(name, "session-") || !strings.HasSuffix(name, ".jsonl") {
		return "", 0, false
	}
	core := strings.TrimSuffix(strings.TrimPrefix(name, "session-"), ".jsonl")
	lastDash := strings.LastIndex(core, "-")
	if lastDash <= 0 || lastDash+1 >= len(core) {
		return "", 0, false
	}
	timestamp = core[:lastDash]
	if len(timestamp) != len("20060102-150405") {
		return "", 0, false
	}
	parsedPID, err := strconv.Atoi(core[lastDash+1:])
	if err != nil {
		return "", 0, false
	}
	return timestamp, parsedPID, true
}

// CleanupOldFiles removes the oldest session log files when the count
// exceeds MaxFiles.
func (s *Service) CleanupOldFiles() {
	s.mu.RLock()
	currentPath := s.path
	s.mu.RUnlock()
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

	// Canonical files are ordered by timestamp and numeric PID.
	sort.Slice(logFiles, func(i, j int) bool {
		leftTS, leftPID, leftOK := parseFileSortKey(logFiles[i])
		rightTS, rightPID, rightOK := parseFileSortKey(logFiles[j])
		if leftOK && rightOK {
			if leftTS != rightTS {
				return leftTS < rightTS
			}
			if leftPID != rightPID {
				return leftPID < rightPID
			}
			return logFiles[i] < logFiles[j]
		}
		if leftOK != rightOK {
			// Prefer trimming malformed/non-canonical files before canonical ones.
			return !leftOK
		}
		return logFiles[i] < logFiles[j]
	})

	excess := len(logFiles) - MaxFiles
	if excess <= 0 {
		return
	}

	deleted := 0
	deleteErrors := 0
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
			deleteErrors++
			continue
		}
		slog.Debug("[session-log] deleted old log file", "path", target)
		deleted++
	}
	if deleted < excess {
		slog.Warn(
			"[session-log] cleanup could not enforce max file count",
			"dir", logDir,
			"maxFiles", MaxFiles,
			"remainingOverLimit", excess-deleted,
			"deleteErrors", deleteErrors,
		)
	}
}

// WriteEntry appends an entry to both the in-memory ring buffer and
// the JSONL file. All state mutations and the shouldEmit decision are made in a
// single lock acquisition to eliminate the race window between the two former
// critical sections.
//
// NOTE(A-0): The event model uses "ping + fetch" -- the emitted event
// "app:session-log-updated" carries no payload. The frontend receives the ping
// and calls Snapshot() to obtain the full snapshot. This eliminates
// data loss from throttling: even if pings are throttled, the next ping triggers
// a full snapshot fetch that includes all entries.
//
// NOTE(A-0b): Sync() is called outside the mutex to prevent disk I/O latency.
// The file pointer is captured under lock and synced after unlock.
func (s *Service) WriteEntry(entry Entry) {
	// NOTE: slog.Warn/Error must NOT be called while mu is held.
	// The TeeHandler intercepts slog records and calls this function back,
	// which would deadlock on the non-reentrant mutex. Internal diagnostics
	// use fmt.Fprintf(os.Stderr, ...) to bypass the TeeHandler entirely.

	var marshalErr, writeErr error
	shouldEmit := false
	var syncFile *os.File

	s.mu.Lock()

	// --- assign monotonic sequence number ---
	s.seq++
	entry.Seq = s.seq

	// --- file persistence ---
	if s.file != nil {
		raw, err := json.Marshal(entry)
		if err != nil {
			marshalErr = err
		} else {
			raw = append(raw, '\n')
			if _, err := s.file.Write(raw); err != nil {
				writeErr = err
			} else if entry.Level == "error" {
				// Capture while holding the lock, then sync after unlock.
				syncFile = s.file
			}
		}
	}

	// --- in-memory ring buffer append ---
	s.entries.push(entry)

	// --- throttle decision (single lock acquisition) ---
	now := time.Now()
	if now.Sub(s.lastEmit) >= emitMinInterval {
		s.lastEmit = now
		shouldEmit = true
	}

	s.mu.Unlock()

	// --- post-lock I/O: Sync() for error-level entries (A-0b) ---
	if syncFile != nil {
		if syncErr := syncFile.Sync(); syncErr != nil {
			// NOTE: os.ErrClosed may occur during shutdown if Close()
			// races with this post-lock Sync(). This is benign -- the file was
			// already flushed and closed by Close().
			// Windows can also return EINVAL when Sync races with file closure.
			isExpectedCloseRace := errors.Is(syncErr, os.ErrClosed) ||
				(runtime.GOOS == "windows" && errors.Is(syncErr, syscall.EINVAL))
			if !isExpectedCloseRace {
				fmt.Fprintf(os.Stderr, "[session-log] failed to sync log file: %v\n", syncErr)
			}
		}
	}

	// Log internal errors via stderr to avoid TeeHandler -> WriteEntry recursion.
	if marshalErr != nil {
		fmt.Fprintf(os.Stderr, "[session-log] failed to marshal log entry: %v\n", marshalErr)
	}
	if writeErr != nil {
		fmt.Fprintf(os.Stderr, "[session-log] failed to write log entry: %v\n", writeErr)
	}

	// NOTE(A-0): Emit lightweight ping outside lock. The frontend calls
	// Snapshot() to fetch the full snapshot on receipt.
	// Throttling only affects the ping frequency, never causes data loss.
	//
	// NOTE: nil is passed as payload, which arrives as JSON null on the frontend.
	// This is intentional -- the event is a notification trigger, not a data carrier.
	if shouldEmit && s.emitter != nil {
		s.emitter.Emit("app:session-log-updated", nil)
	}
}

// Close flushes and closes the session log file handle.
func (s *Service) Close() {
	var closeErr error

	s.mu.Lock()
	if s.file != nil {
		closeErr = s.file.Close()
		s.file = nil
	}
	s.mu.Unlock()

	if closeErr != nil {
		fmt.Fprintf(os.Stderr, "[session-log] failed to close log file: %v\n", closeErr)
	}
}

// Snapshot returns a copy of all in-memory session log entries.
// Uses RLock because this is a read-only operation on the ring buffer.
func (s *Service) Snapshot() []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.entries.snapshot()
}

// FilePath returns the absolute path to the current session's JSONL log file.
// Uses RLock because this is a read-only operation.
func (s *Service) FilePath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.path
}

// NormalizeLogLevel maps an arbitrary level string to one of the canonical
// values used by the session log system: "error", "warn", "debug", or "info".
// Any unrecognized value maps to "info" as a safe default.
func NormalizeLogLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "error":
		return "error"
	case "warn", "warning":
		return "warn"
	case "debug":
		return "debug"
	default:
		return "info"
	}
}

// TruncateRunes returns s capped to maxRunes runes without allocating when
// s is already within the limit.
func TruncateRunes(s string, maxRunes int) string {
	if maxRunes <= 0 || s == "" {
		return ""
	}
	runeCount := 0
	for byteIndex := range s {
		if runeCount == maxRunes {
			return s[:byteIndex]
		}
		runeCount++
	}
	return s
}

// LogFrontendEvent writes a frontend-originated event into the session log.
//
// Inputs are sanitized before storage:
//   - level is normalized to "error", "warn", "debug", or "info" via NormalizeLogLevel.
//   - msg and source are trimmed and capped by rune count.
//   - Empty msg after trimming is silently discarded.
func (s *Service) LogFrontendEvent(level, msg, source string) {
	level = NormalizeLogLevel(level)
	msg = strings.TrimSpace(msg)
	source = strings.TrimSpace(source)
	if msg == "" {
		// NOTE: Silently discard empty messages. LogFrontendEvent is a
		// fire-and-forget Wails call with no return value; callers cannot
		// observe the discard.
		return
	}
	msg = TruncateRunes(msg, FrontendLogMaxMsgLen)
	source = TruncateRunes(source, FrontendLogMaxSourceLen)
	s.WriteEntry(Entry{
		Timestamp: time.Now().Format("20060102150405"),
		Level:     level,
		Message:   msg,
		Source:    source,
	})
}
