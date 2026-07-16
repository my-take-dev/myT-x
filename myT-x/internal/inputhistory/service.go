package inputhistory

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"myT-x/internal/apptypes"
	"myT-x/internal/sessioninfo"
)

// Service manages input history persistence, buffering, and event emission.
type Service struct {
	emitter        apptypes.RuntimeEventEmitter
	isShuttingDown func() bool
	resolveWorkDir func(sessionName string) (string, error)
	configDir      func() (string, error)
	now            func() time.Time
	cleanupDelay   time.Duration
	mu             sync.RWMutex
	scopes         map[string]*scopeState
	lastPath       string
	file           *os.File
	path           string
	entries        ringBuffer
	lastEmit       time.Time
	seq            uint64
	cleanupMu      sync.Mutex
	cleanupStop    chan struct{}
	cleanupDone    chan struct{}
	lineBufMu      sync.Mutex
	lineBuffers    map[string]*lineBuffer
}

// NewService creates a new input history service.
func NewService(emitter apptypes.RuntimeEventEmitter, isShuttingDown func() bool, options ...Option) *Service {
	if isShuttingDown == nil {
		isShuttingDown = func() bool { return false }
	}
	opts := serviceOptions{
		now:          time.Now,
		cleanupDelay: cleanupDelay,
	}
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}
	if opts.now == nil {
		opts.now = time.Now
	}
	return &Service{
		emitter:        emitter,
		isShuttingDown: isShuttingDown,
		resolveWorkDir: opts.resolveSessionWorkDir,
		configDir:      opts.configDir,
		now:            opts.now,
		cleanupDelay:   opts.cleanupDelay,
		scopes:         map[string]*scopeState{},
		entries:        newRingBuffer(maxEntries),
		lineBuffers:    map[string]*lineBuffer{},
	}
}

func writeDiagnostic(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format, args...)
}

// Init creates the JSONL input history file for the current run.
// Non-fatal: logs a warning and continues if any I/O operation fails.
//
// If called more than once (re-initialization), pending line buffers are
// flushed before switching to the new file. The previous file handle is
// closed after the lock is released to avoid holding the lock during I/O.
func (s *Service) Init(configPath string) {
	// Flush any pending line buffers before switching files to prevent data loss.
	s.FlushAllLineBuffers()

	configDir := filepath.Dir(configPath)
	if s.configDir == nil && strings.TrimSpace(configDir) != "" {
		s.configDir = func() (string, error) { return configDir, nil }
	}
	if s.resolveWorkDir != nil {
		s.scheduleCleanup()
		slog.Info("[input-history] initialized session-scoped storage", "configDir", configDir)
		return
	}

	dir := filepath.Join(filepath.Dir(configPath), Dir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		slog.Warn("[input-history] failed to create history directory", "dir", dir, "error", err)
		return
	}

	filename := fmt.Sprintf("input-%s-%d.jsonl", time.Now().Format("20060102-150405"), os.Getpid())
	fullPath := filepath.Join(dir, filename)

	f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		slog.Warn("[input-history] failed to open history file", "path", fullPath, "error", err)
		return
	}

	var oldFile *os.File
	s.mu.Lock()
	oldFile = s.file
	s.file = f
	s.path = fullPath
	s.mu.Unlock()

	if oldFile != nil {
		if err := oldFile.Close(); err != nil {
			slog.Warn("[input-history] failed to close previous history file", "error", err)
		}
	}

	s.CleanupOldFiles()
	slog.Info("[input-history] initialized", "path", fullPath)
}

func (s *Service) scheduleCleanup() {
	delay := max(s.cleanupDelay, 0)
	s.cleanupMu.Lock()
	if s.cleanupStop != nil {
		s.cleanupMu.Unlock()
		return
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	s.cleanupStop = stop
	s.cleanupDone = done
	s.cleanupMu.Unlock()

	go func() {
		defer close(done)

		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
			if !s.isShuttingDown() {
				s.CleanupOldFiles()
			}
		case <-stop:
			return
		}

		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if !s.isShuttingDown() {
					s.CleanupOldFiles()
				}
			case <-stop:
				return
			}
		}
	}()
}

func (s *Service) stopCleanup() {
	s.cleanupMu.Lock()
	stop := s.cleanupStop
	done := s.cleanupDone
	s.cleanupStop = nil
	s.cleanupDone = nil
	s.cleanupMu.Unlock()

	if stop == nil {
		return
	}
	close(stop)
	<-done
}

func isAllDecimalDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func parseFileName(name string) (timestamp string, pid int, ok bool) {
	if !strings.HasPrefix(name, "input-") || !strings.HasSuffix(name, ".jsonl") {
		return "", 0, false
	}
	core := strings.TrimSuffix(strings.TrimPrefix(name, "input-"), ".jsonl")
	parts := strings.Split(core, "-")
	if len(parts) != 3 {
		return "", 0, false
	}
	datePart, timePart, pidPart := parts[0], parts[1], parts[2]
	if len(datePart) != 8 || len(timePart) != 6 {
		return "", 0, false
	}
	if !isAllDecimalDigits(datePart) || !isAllDecimalDigits(timePart) || !isAllDecimalDigits(pidPart) {
		return "", 0, false
	}
	parsedPID, err := strconv.Atoi(pidPart)
	if err != nil {
		return "", 0, false
	}
	return datePart + "-" + timePart, parsedPID, true
}

func sortFilesForCleanup(files []string) {
	sort.Slice(files, func(i, j int) bool {
		leftName, rightName := files[i], files[j]
		leftTS, leftPID, leftOK := parseFileName(leftName)
		rightTS, rightPID, rightOK := parseFileName(rightName)
		switch {
		case leftOK && rightOK:
			if leftTS != rightTS {
				return leftTS < rightTS
			}
			if leftPID != rightPID {
				return leftPID < rightPID
			}
			return leftName < rightName
		case leftOK != rightOK:
			return !leftOK
		default:
			return leftName < rightName
		}
	})
}

// CleanupOldFiles removes the oldest input history files when the count exceeds MaxFiles.
func (s *Service) CleanupOldFiles() {
	if s.resolveWorkDir != nil {
		configDir, err := s.currentConfigDir()
		if err != nil {
			slog.Warn("[input-history] cleanup skipped: config dir unavailable", "error", err)
			return
		}
		if err := cleanupSessionInfoHistoryFiles(configDir, s.now()); err != nil {
			slog.Warn("[input-history] session-info cleanup failed", "error", err)
		}
		return
	}

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
		slog.Warn("[input-history] failed to read history directory for cleanup", "dir", logDir, "error", err)
		return
	}

	var histFiles []string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && strings.HasPrefix(name, "input-") && strings.HasSuffix(name, ".jsonl") {
			histFiles = append(histFiles, name)
		}
	}

	sortFilesForCleanup(histFiles)

	excess := len(histFiles) - MaxFiles
	if excess <= 0 {
		return
	}

	deleted := 0
	deleteErrors := 0
	for _, name := range histFiles {
		if deleted >= excess {
			break
		}
		if name == currentFile {
			continue
		}
		target := filepath.Join(logDir, name)
		if err := os.Remove(target); err != nil {
			slog.Warn("[input-history] failed to delete old history file", "path", target, "error", err)
			deleteErrors++
			continue
		}
		slog.Debug("[input-history] deleted old history file", "path", target)
		deleted++
	}

	if deleted < excess {
		remainingOverLimit := excess - deleted
		slog.Warn(
			"[input-history] cleanup could not enforce max file count",
			"dir", logDir,
			"maxFiles", MaxFiles,
			"remainingOverLimit", remainingOverLimit,
			"deleteErrors", deleteErrors,
		)
	}
}

// ProcessInputString strips terminal escape sequences from raw input,
// preserving only printable characters and specific control characters
// needed for line-editing semantics.
//
// Removed sequences:
//   - CSI (ESC [): cursor movement, SGR attributes, erase commands
//   - OSC (ESC ]): title sets, hyperlinks (terminated by BEL or ST)
//   - DCS (ESC P), SOS (ESC X), PM (ESC ^), APC (ESC _): device/app sequences (terminated by ST)
//   - SS3 (ESC O): function key sequences (single character after ESC O)
//   - Other control characters (newline, tab, etc.)
//
// Preserved control characters:
//   - \r (0x0D): Enter/carriage return — marks line completion
//   - \x03: Ctrl+C — interrupt signal, recorded as "^C"
//   - \x04: Ctrl+D — EOF signal, recorded as "^D"
//   - \x08: Backspace — removes last character from buffer
//   - \x7f: DEL — same as backspace
func ProcessInputString(input string) string {
	if input == "" {
		return ""
	}
	runes := []rune(input)
	var out strings.Builder

	skipEscString := func(start int, allowBEL bool) int {
		for idx := start; idx < len(runes); idx++ {
			if allowBEL && runes[idx] == '\x07' {
				return idx
			}
			if runes[idx] == '\x1b' && idx+1 < len(runes) && runes[idx+1] == '\\' {
				return idx + 1
			}
		}
		return len(runes)
	}

	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\x1b' {
			if i+1 >= len(runes) {
				continue
			}
			switch runes[i+1] {
			case '[':
				i += 2
				for i < len(runes) && !(runes[i] >= 0x40 && runes[i] <= 0x7e) {
					i++
				}
			case ']':
				i = skipEscString(i+2, true)
			case 'P', 'X', '^', '_':
				i = skipEscString(i+2, false)
			case 'O':
				i += 2
			default:
			}
			continue
		}

		if r == '\r' || r == '\x03' || r == '\x04' || r == '\x08' || r == '\x7f' {
			out.WriteRune(r)
			continue
		}
		if !unicode.IsControl(r) {
			out.WriteRune(r)
		}
	}
	return out.String()
}

// RecordInput processes raw terminal input and appends complete command lines
// to the history when Enter is received.
//
// Buffering rules per character:
//   - \r (Enter): commits the current buffer as a history entry, clears the buffer
//   - \x03 (Ctrl+C): discards the buffer, records "^C" as a separate entry
//   - \x04 (Ctrl+D): records "^D" (empty buffer) or "text (^D)" (non-empty), clears the buffer
//   - \x08, \x7f (Backspace/DEL): removes the last rune from the buffer
//   - Other printable: appends to the buffer, resets the inactivity flush timer
//
// When no Enter is received within lineFlushTimeout, the partial buffer is
// flushed as-is (handles password prompts, interactive modes, etc.).
func (s *Service) RecordInput(paneID, input, source, session string) {
	if input == "" {
		return
	}
	if s.isShuttingDown() {
		return
	}
	cleaned := ProcessInputString(input)
	if cleaned == "" {
		return
	}

	type pendingEntry struct {
		text    string
		source  string
		session string
	}
	var toWrite []pendingEntry

	s.lineBufMu.Lock()
	if s.lineBuffers == nil {
		s.lineBuffers = make(map[string]*lineBuffer)
	}
	lb, exists := s.lineBuffers[paneID]
	if !exists {
		lb = &lineBuffer{paneID: paneID, source: source, session: session}
		s.lineBuffers[paneID] = lb
	}
	if source != "" || lb.source == "" {
		lb.source = source
	}
	if session != "" || lb.session == "" {
		lb.session = session
	}

	for _, r := range cleaned {
		switch r {
		case '\r':
			text := string(lb.buf)
			lb.buf = lb.buf[:0]
			lb.stopTimer()
			if text != "" {
				toWrite = append(toWrite, pendingEntry{text: text, source: lb.source, session: lb.session})
			}

		case '\x03':
			lb.buf = lb.buf[:0]
			lb.stopTimer()
			toWrite = append(toWrite, pendingEntry{text: "^C", source: lb.source, session: lb.session})

		case '\x04':
			text := string(lb.buf)
			lb.buf = lb.buf[:0]
			lb.stopTimer()
			entryText := "^D"
			if text != "" {
				entryText = text + " (^D)"
			}
			toWrite = append(toWrite, pendingEntry{text: entryText, source: lb.source, session: lb.session})

		case '\x08', '\x7f':
			if len(lb.buf) > 0 {
				lb.buf = lb.buf[:len(lb.buf)-1]
			}
			if len(lb.buf) > 0 {
				s.resetLineBufferTimer(lb)
			} else {
				lb.stopTimer()
			}

		default:
			if len(lb.buf) >= MaxInputLen {
				slog.Debug("[input-history] input truncated: max rune limit reached", "paneID", paneID)
				continue
			}
			lb.buf = append(lb.buf, r)
			s.resetLineBufferTimer(lb)
		}
	}
	s.lineBufMu.Unlock()

	ts := time.Now().Format("20060102150405")
	for _, p := range toWrite {
		s.WriteEntry(Entry{
			Timestamp: ts,
			PaneID:    paneID,
			Input:     p.text,
			Source:    p.source,
			Session:   p.session,
		})
	}
}

// resetLineBufferTimer restarts the inactivity flush timer for a line buffer.
//
// Uses a generation counter pattern to prevent stale timer callbacks:
// each new timer captures the current generation value. When the timer fires,
// FlushLineBuffer compares the captured generation against the buffer's current
// timerGen — if they differ, the callback is stale (input was received after
// the timer was set) and the flush is skipped.
func (s *Service) resetLineBufferTimer(lb *lineBuffer) {
	if lb.timer != nil {
		lb.timer.Stop()
	}
	lb.timerGen++
	gen := lb.timerGen
	paneID := lb.paneID
	lb.timer = time.AfterFunc(lineFlushTimeout, func() {
		if s.isShuttingDown() {
			return
		}
		s.FlushLineBuffer(paneID, gen)
	})
}

// FlushLineBuffer extracts and writes any pending buffered text for the given pane.
//
// The gen parameter controls staleness detection:
//   - Normal timer callback: gen equals the timerGen captured when the timer was set.
//     If lb.timerGen has since changed (new input arrived), the flush is skipped.
//   - ShutdownFlushSentinel: bypasses the generation check entirely, forcing a flush
//     regardless of timer state. Used during graceful shutdown.
func (s *Service) FlushLineBuffer(paneID string, gen uint64) {
	if gen != ShutdownFlushSentinel && s.isShuttingDown() {
		return
	}

	s.lineBufMu.Lock()
	lb, exists := s.lineBuffers[paneID]
	if !exists || lb == nil {
		s.lineBufMu.Unlock()
		return
	}
	if gen != ShutdownFlushSentinel && lb.timerGen != gen {
		s.lineBufMu.Unlock()
		return
	}
	if gen == ShutdownFlushSentinel {
		lb.stopTimer()
	}

	text := string(lb.buf)
	if text == "" {
		lb.timer = nil
		s.lineBufMu.Unlock()
		return
	}
	lb.buf = lb.buf[:0]
	lb.timer = nil
	source := lb.source
	session := lb.session
	s.lineBufMu.Unlock()

	s.WriteEntry(Entry{
		Timestamp: time.Now().Format("20060102150405"),
		PaneID:    paneID,
		Input:     text,
		Source:    source,
		Session:   session,
	})
}

// FlushAllLineBuffers stops all pending timers and writes any buffered text.
func (s *Service) FlushAllLineBuffers() {
	s.lineBufMu.Lock()
	if s.lineBuffers == nil {
		s.lineBufMu.Unlock()
		return
	}

	type pendingEntry struct {
		paneID  string
		text    string
		source  string
		session string
	}
	pending := make([]pendingEntry, 0, len(s.lineBuffers))
	for paneID, lb := range s.lineBuffers {
		if lb == nil {
			continue
		}
		lb.stopTimer()
		if text := string(lb.buf); text != "" {
			pending = append(pending, pendingEntry{paneID: paneID, text: text, source: lb.source, session: lb.session})
		}
	}
	s.lineBuffers = nil
	s.lineBufMu.Unlock()

	ts := time.Now().Format("20060102150405")
	for _, p := range pending {
		s.WriteEntry(Entry{
			Timestamp: ts,
			PaneID:    p.paneID,
			Input:     p.text,
			Source:    p.source,
			Session:   p.session,
		})
	}
}

// WriteEntry appends an entry to both the in-memory ring buffer and the JSONL file.
// Typically called via RecordInput (line-buffered path) or FlushLineBuffer
// (timeout/shutdown path). Direct calls are reserved for flush operations.
//
// Design notes:
//   - Event model ("ping + fetch"): the emitted "app:input-history-updated" event
//     carries no payload. The frontend receives the ping and re-fetches the full
//     history via GetInputHistory(). This decouples the write path from serialization
//     format concerns and avoids double-encoding.
//   - Sync() intentionally omitted: input history is high-frequency, non-critical data.
//     The fsync cost per write would degrade interactive responsiveness. Acceptable
//     trade-off: up to ~5 seconds of history may be lost on unclean shutdown.
//   - writeDiagnostic(...) used instead of slog.Warn: WriteEntry may be called
//     from TeeHandler's slog log handler (via RecordInput). Using slog here would
//     cause recursive locking inside the slog handler chain.
func (s *Service) WriteEntry(entry Entry) {
	if s.resolveWorkDir == nil {
		s.writeLegacyEntry(entry)
		return
	}

	scope, err := s.resolveScope(entry.Session)
	if err != nil {
		writeDiagnostic("[input-history] dropped entry: resolve scope for session %q: %v\n", entry.Session, err)
		return
	}

	if err := s.ensureScopeLoaded(scope); err != nil {
		writeDiagnostic("[input-history] dropped entry: load scope %q: %v\n", scope.key, err)
		return
	}

	shouldEmit := false

	s.mu.Lock()

	if err := s.ensureDailyFileLocked(scope); err != nil {
		s.mu.Unlock()
		writeDiagnostic("[input-history] dropped entry: open daily file for scope %q: %v\n", scope.key, err)
		return
	}

	nextSeq := scope.seq + 1
	entry.Seq = nextSeq

	raw, err := json.Marshal(entry)
	if err != nil {
		s.mu.Unlock()
		writeDiagnostic("[input-history] failed to marshal entry: %v\n", err)
		return
	}
	raw = append(raw, '\n')
	if _, err := scope.file.Write(raw); err != nil {
		s.mu.Unlock()
		writeDiagnostic("[input-history] failed to write entry: %v\n", err)
		return
	}

	scope.seq = nextSeq
	scope.entries.push(entry)
	s.lastPath = scope.path

	now := s.now()
	if now.Sub(s.lastEmit) >= emitMinInterval {
		s.lastEmit = now
		shouldEmit = true
	}

	s.mu.Unlock()

	if shouldEmit && s.emitter != nil {
		s.emitter.Emit("app:input-history-updated", nil)
	}
}

func (s *Service) writeLegacyEntry(entry Entry) {
	var marshalErr, writeErr error
	shouldEmit := false

	s.mu.Lock()

	s.seq++
	entry.Seq = s.seq

	if s.file != nil {
		raw, err := json.Marshal(entry)
		if err != nil {
			marshalErr = err
		} else {
			raw = append(raw, '\n')
			if _, err := s.file.Write(raw); err != nil {
				writeErr = err
			}
		}
	}

	s.entries.push(entry)

	now := time.Now()
	if now.Sub(s.lastEmit) >= emitMinInterval {
		s.lastEmit = now
		shouldEmit = true
	}

	s.mu.Unlock()

	if marshalErr != nil {
		writeDiagnostic("[input-history] failed to marshal entry: %v\n", marshalErr)
	}
	if writeErr != nil {
		writeDiagnostic("[input-history] failed to write entry: %v\n", writeErr)
	}

	if shouldEmit && s.emitter != nil {
		s.emitter.Emit("app:input-history-updated", nil)
	}
}

// Close closes the input history file handle.
// Callers MUST call FlushAllLineBuffers() before Close() to avoid losing
// pending buffered input. The standard shutdown sequence in app_lifecycle.go
// is: FlushAllLineBuffers() → Close().
func (s *Service) Close() {
	s.stopCleanup()

	var closeErrors []error

	s.mu.Lock()
	for _, scope := range s.scopes {
		if scope != nil && scope.file != nil {
			if err := scope.file.Close(); err != nil {
				closeErrors = append(closeErrors, err)
			}
			scope.file = nil
		}
	}
	if s.file != nil {
		if err := s.file.Close(); err != nil {
			closeErrors = append(closeErrors, err)
		}
		s.file = nil
	}
	s.mu.Unlock()

	for _, closeErr := range closeErrors {
		writeDiagnostic("[input-history] failed to close history file: %v\n", closeErr)
	}
}

// Snapshot returns a copy of all in-memory input history entries.
func (s *Service) Snapshot() []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.entries.snapshot()
}

// SnapshotForSession returns input history scoped to the session's work directory.
func (s *Service) SnapshotForSession(sessionName string) Snapshot {
	if s.resolveWorkDir == nil {
		return Snapshot{Entries: s.Snapshot()}
	}
	scope, err := s.resolveScope(sessionName)
	if err != nil {
		slog.Warn("[input-history] failed to resolve input history scope", "session", sessionName, "error", err)
		return Snapshot{Entries: []Entry{}}
	}

	if err := s.ensureScopeLoaded(scope); err != nil {
		slog.Warn("[input-history] failed to load input history scope", "scopeKey", scope.key, "error", err)
		return Snapshot{ScopeKey: scope.key, Entries: []Entry{}}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return Snapshot{ScopeKey: scope.key, Entries: scope.entries.snapshot()}
}

// FilePath returns the current history file path.
func (s *Service) FilePath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.lastPath != "" {
		return s.lastPath
	}
	return s.path
}

// FilePathForSession returns the daily history path for the session scope.
func (s *Service) FilePathForSession(sessionName string) string {
	if s.resolveWorkDir == nil {
		return s.FilePath()
	}
	scope, err := s.resolveScope(sessionName)
	if err != nil {
		slog.Warn("[input-history] failed to resolve input history file path", "session", sessionName, "error", err)
		return ""
	}

	s.mu.RLock()
	path := scope.path
	dir := scope.dir
	s.mu.RUnlock()
	if path != "" {
		return path
	}
	return filepath.Join(dir, fmt.Sprintf("input-%s.jsonl", s.now().Format("20060102")))
}

func (s *Service) currentConfigDir() (string, error) {
	if s.configDir != nil {
		configDir, err := s.configDir()
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(configDir) == "" {
			return "", fmt.Errorf("config dir is empty")
		}
		return configDir, nil
	}
	return "", fmt.Errorf("config dir resolver is not configured")
}

func (s *Service) resolveScope(sessionName string) (*scopeState, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		configDir, err := s.currentConfigDir()
		if err != nil {
			return nil, err
		}
		return s.scopeForDir("", filepath.Join(filepath.Clean(configDir), Dir)), nil
	}
	workDir, err := s.resolveWorkDir(sessionName)
	if err != nil {
		return nil, err
	}
	configDir, err := s.currentConfigDir()
	if err != nil {
		return nil, err
	}
	key, err := sessioninfo.FolderKey(workDir)
	if err != nil {
		return nil, err
	}
	baseDir, err := sessioninfo.DirectoryPath(configDir, workDir)
	if err != nil {
		return nil, err
	}
	return s.scopeForDir(key, filepath.Join(baseDir, Dir)), nil
}

func (s *Service) scopeForDir(key, historyDir string) *scopeState {
	s.mu.RLock()
	scope := s.scopes[key]
	s.mu.RUnlock()
	if scope != nil && scope.dir == historyDir {
		return scope
	}

	var staleFile *os.File
	s.mu.Lock()
	if scope = s.scopes[key]; scope != nil {
		if scope.dir == historyDir {
			s.mu.Unlock()
			return scope
		}
		staleFile = scope.file
		scope.file = nil
		scope.path = ""
		scope.fileDate = ""
		scope.dir = historyDir
		s.mu.Unlock()
		if staleFile != nil {
			if err := staleFile.Close(); err != nil {
				writeDiagnostic("[input-history] failed to close stale scope file for scope %q: %v\n", key, err)
			}
		}
		return scope
	}

	scope = &scopeState{
		key:     key,
		dir:     historyDir,
		entries: newRingBuffer(maxEntries),
	}
	s.scopes[key] = scope
	s.mu.Unlock()
	return scope
}

func (s *Service) ensureScopeLoaded(scope *scopeState) error {
	s.mu.RLock()
	if scope.loaded {
		s.mu.RUnlock()
		return nil
	}
	dir := scope.dir
	s.mu.RUnlock()

	entries, maxSeq, loadErrors, err := s.loadScopeFromDisk(dir)
	for _, loadErr := range loadErrors {
		writeDiagnostic("[input-history] failed to load daily history file: %v\n", loadErr)
	}
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if scope.loaded {
		return nil
	}
	scope.entries = entries
	if maxSeq > scope.seq {
		scope.seq = maxSeq
	}
	scope.loaded = true
	return nil
}

func (s *Service) loadScopeFromDisk(dir string) (ringBuffer, uint64, []error, error) {
	entries := newRingBuffer(maxEntries)
	files, err := listDailyFiles(dir)
	if err != nil {
		return entries, 0, nil, err
	}

	var loadErrors []error
	var maxSeq uint64
	minDate := dateOnly(s.now()).AddDate(0, 0, -(LoadWindowDays - 1))
	for _, file := range files {
		if file.date.Before(minDate) {
			continue
		}
		nextSeq, err := loadDailyFile(&entries, file.path, maxSeq)
		maxSeq = nextSeq
		if err != nil {
			loadErrors = append(loadErrors, err)
		}
	}
	return entries, maxSeq, loadErrors, nil
}

func loadDailyFile(entries *ringBuffer, path string, maxSeq uint64) (uint64, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return maxSeq, nil
		}
		return maxSeq, err
	}
	defer func() {
		if err := file.Close(); err != nil {
			writeDiagnostic("[input-history] failed to close history file after load %q: %v\n", path, err)
		}
	}()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var entry Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			writeDiagnostic("[input-history] skipped malformed history line in %q: %v\n", path, err)
			continue
		}
		if entry.Seq == 0 {
			maxSeq++
			entry.Seq = maxSeq
		} else if entry.Seq > maxSeq {
			maxSeq = entry.Seq
		}
		entries.push(entry)
	}
	if err := scanner.Err(); err != nil {
		return maxSeq, err
	}
	return maxSeq, nil
}

func (s *Service) ensureDailyFileLocked(scope *scopeState) error {
	date := s.now().Format("20060102")
	if scope.file != nil && scope.fileDate == date {
		return nil
	}
	if err := os.MkdirAll(scope.dir, 0o700); err != nil {
		return fmt.Errorf("create input history directory: %w", err)
	}
	if scope.file != nil {
		if err := scope.file.Close(); err != nil {
			writeDiagnostic("[input-history] failed to close previous daily file for scope %q: %v\n", scope.key, err)
		}
		scope.file = nil
	}
	path := filepath.Join(scope.dir, fmt.Sprintf("input-%s.jsonl", date))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	scope.file = file
	scope.path = path
	scope.fileDate = date
	return nil
}

type dailyFile struct {
	path string
	date time.Time
}

func listDailyFiles(dir string) ([]dailyFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	files := make([]dailyFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		date, ok := parseDailyFileName(entry.Name())
		if !ok {
			continue
		}
		files = append(files, dailyFile{path: filepath.Join(dir, entry.Name()), date: date})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].date.Before(files[j].date)
	})
	return files, nil
}

func parseDailyFileName(name string) (time.Time, bool) {
	if !strings.HasPrefix(name, "input-") || !strings.HasSuffix(name, ".jsonl") {
		return time.Time{}, false
	}
	datePart := strings.TrimSuffix(strings.TrimPrefix(name, "input-"), ".jsonl")
	if len(datePart) != 8 || !isAllDecimalDigits(datePart) {
		return time.Time{}, false
	}
	// Daily history files are keyed by the user's local calendar day.
	date, err := time.ParseInLocation("20060102", datePart, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return date, true
}

func cleanupSessionInfoHistoryFiles(configDir string, now time.Time) error {
	absoluteConfigDir, err := filepath.Abs(strings.TrimSpace(configDir))
	if err != nil {
		return fmt.Errorf("resolve config dir: %w", err)
	}
	absoluteConfigDir = filepath.Clean(absoluteConfigDir)
	cutoff := dateOnly(now).AddDate(0, 0, -RetentionDays)

	cleanupScopedHistoryDir(filepath.Join(absoluteConfigDir, Dir), cutoff)

	root := filepath.Join(absoluteConfigDir, sessioninfo.DirName)
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		cleanupScopedHistoryDir(filepath.Join(root, entry.Name(), Dir), cutoff)
	}
	return nil
}

func cleanupScopedHistoryDir(historyDir string, cutoff time.Time) {
	files, err := listDailyFiles(historyDir)
	if err != nil {
		slog.Warn("[input-history] failed to list scoped history directory", "dir", historyDir, "error", err)
		return
	}
	for _, file := range files {
		if !file.date.Before(cutoff) {
			continue
		}
		if err := os.Remove(file.path); err != nil {
			slog.Warn("[input-history] failed to delete expired history file", "path", file.path, "error", err)
		}
	}
	_ = os.Remove(historyDir)
	_ = os.Remove(filepath.Dir(historyDir))
}

func dateOnly(t time.Time) time.Time {
	year, month, day := t.Local().Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.Local)
}
