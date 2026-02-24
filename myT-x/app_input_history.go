package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// initInputHistory creates the JSONL input history file for the current run.
// Non-fatal: logs a warning and continues if any I/O operation fails.
func (a *App) initInputHistory() {
	dir := filepath.Join(filepath.Dir(a.configPath), inputHistoryDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		slog.Warn("[input-history] failed to create history directory", "dir", dir, "error", err)
		return
	}

	// NOTE: PID is appended to prevent filename collision on sub-second restart.
	filename := fmt.Sprintf("input-%s-%d.jsonl", time.Now().Format("20060102-150405"), os.Getpid())
	fullPath := filepath.Join(dir, filename)

	f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		slog.Warn("[input-history] failed to open history file", "path", fullPath, "error", err)
		return
	}

	a.inputHistoryMu.Lock()
	a.inputHistoryFile = f
	a.inputHistoryPath = fullPath
	a.inputHistoryMu.Unlock()

	a.cleanupOldInputHistory()

	slog.Info("[input-history] initialized", "path", fullPath)
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

func parseInputHistoryFileName(name string) (timestamp string, pid int, ok bool) {
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

func sortInputHistoryFilesForCleanup(files []string) {
	sort.Slice(files, func(i, j int) bool {
		leftName, rightName := files[i], files[j]
		leftTS, leftPID, leftOK := parseInputHistoryFileName(leftName)
		rightTS, rightPID, rightOK := parseInputHistoryFileName(rightName)
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
			// Non-standard names are sorted first so cleanup can trim malformed
			// files before touching canonical timestamp/PID files.
			return !leftOK
		default:
			return leftName < rightName
		}
	})
}

// cleanupOldInputHistory removes the oldest input history files when the count
// exceeds inputHistoryMaxFiles.
func (a *App) cleanupOldInputHistory() {
	a.inputHistoryMu.RLock()
	currentPath := a.inputHistoryPath
	a.inputHistoryMu.RUnlock()
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

	// Canonical files are ordered by timestamp and numeric PID.
	// This avoids lexicographic PID mis-ordering in same-second files
	// (e.g. "...-10.jsonl" appearing older than "...-9.jsonl").
	sortInputHistoryFilesForCleanup(histFiles)

	excess := len(histFiles) - inputHistoryMaxFiles
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
		// Windows can deny remove for files still open in another process.
		// Keep running and report the over-limit remainder as best-effort cleanup.
		slog.Warn(
			"[input-history] cleanup could not enforce max file count",
			"dir", logDir,
			"maxFiles", inputHistoryMaxFiles,
			"remainingOverLimit", remainingOverLimit,
			"deleteErrors", deleteErrors,
		)
	}
}

// processInputString strips terminal escape sequences from raw input, keeping
// only printable characters and the control characters that recordInput handles.
//
// Removed: CSI sequences (ESC [), OSC sequences (ESC ]), and all other ESC sequences.
// Kept:    printable chars (>= 0x20), \r, \x03 (Ctrl+C), \x04 (Ctrl+D),
//
//	\x08 (Backspace), \x7f (DEL).
//
// Pure function - no side effects, safe to call without locks.
func processInputString(input string) string {
	if input == "" {
		return ""
	}
	runes := []rune(input)
	var out strings.Builder
	// skipEscString scans for the string terminator (ST = ESC \) or optionally BEL (\x07).
	// Returns the index of the terminator's last byte. The caller's loop increment
	// advances past it, so returning idx+1 for ST (two-byte sequence) is correct.
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
			// ESC: consume escape sequence, produce no output.
			if i+1 >= len(runes) {
				continue // lone ESC at end of input
			}
			switch runes[i+1] {
			case '[': // CSI: ESC [ <params> <final>
				i += 2
				for i < len(runes) && !(runes[i] >= 0x40 && runes[i] <= 0x7e) {
					i++
				}
				// i now points to the final byte (consumed by outer loop increment)
			case ']': // OSC: ESC ] <string> BEL|ST
				i = skipEscString(i+2, true)
			case 'P', 'X', '^', '_': // DCS/SOS/PM/APC: ESC <char> <data> ST
				i = skipEscString(i+2, false)
			case 'O': // SS3: ESC O <final> (function keys F1-F4)
				i += 2
				// skip one SS3 final byte; outer loop increment handles it
			default:
				// Lone ESC or unrecognized: skip only the ESC itself.
				// The following character is not consumed - it may be real user input.
			}
			continue
		}
		// Keep printable chars (0x20-0x7e, 0x80+) and specific control chars.
		if r == '\r' || r == '\x03' || r == '\x04' || r == '\x08' || r == '\x7f' {
			out.WriteRune(r)
			continue
		}
		if !unicode.IsControl(r) {
			out.WriteRune(r)
		}
		// All other control chars (0x00-0x1f except the four above) are discarded.
	}
	return out.String()
}

// recordInput processes raw terminal input and appends complete command lines
// to the history when Enter (\r) is received.
//
// Line buffering rules:
//   - \r          -> flush buffer as one entry (empty buffer -> no entry)
//   - \x03 (^C)   -> discard buffer, record "^C"
//   - \x04 (^D)   -> record buffer text + " (^D)" or just "^D" if empty
//   - \x08/\x7f   -> delete last rune from buffer
//   - other ctrl  -> skip (filtered by processInputString before reaching here)
//   - printable   -> append to buffer; flush on timeout (inputLineFlushTimeout)
//
// Lock ordering: inputLineBufMu -> (release) -> inputHistoryMu.
// inputLineBufMu is NEVER held while calling writeInputHistoryEntry.
func (a *App) recordInput(paneID, input, source, session string) {
	if input == "" {
		return
	}
	if a.shuttingDown.Load() {
		return
	}
	cleaned := processInputString(input)
	if cleaned == "" {
		return
	}

	// toWrite collects entries to be written after releasing inputLineBufMu.
	type pendingEntry struct {
		text    string
		source  string
		session string
	}
	var toWrite []pendingEntry

	a.inputLineBufMu.Lock()

	if a.inputLineBuffers == nil {
		a.inputLineBuffers = make(map[string]*inputLineBuffer)
	}
	lb, exists := a.inputLineBuffers[paneID]
	if !exists {
		lb = &inputLineBuffer{paneID: paneID, source: source, session: session}
		a.inputLineBuffers[paneID] = lb
	}
	// Keep metadata aligned with the latest event for this in-flight line.
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
			lb.buf = lb.buf[:0] // retain backing array capacity to reduce allocations
			if lb.timer != nil {
				lb.timer.Stop()
				lb.timer = nil
			}
			if text != "" {
				toWrite = append(toWrite, pendingEntry{
					text:    text,
					source:  lb.source,
					session: lb.session,
				})
			}

		case '\x03': // Ctrl+C: discard line buffer, record ^C
			lb.buf = lb.buf[:0]
			if lb.timer != nil {
				lb.timer.Stop()
				lb.timer = nil
			}
			toWrite = append(toWrite, pendingEntry{
				text:    "^C",
				source:  lb.source,
				session: lb.session,
			})

		case '\x04': // Ctrl+D: record buffer (if any) with ^D annotation
			text := string(lb.buf)
			lb.buf = lb.buf[:0]
			if lb.timer != nil {
				lb.timer.Stop()
				lb.timer = nil
			}
			entryText := "^D"
			if text != "" {
				entryText = text + " (^D)"
			}
			toWrite = append(toWrite, pendingEntry{
				text:    entryText,
				source:  lb.source,
				session: lb.session,
			})

		case '\x08', '\x7f': // Backspace / DEL: remove last rune
			if len(lb.buf) > 0 {
				lb.buf = lb.buf[:len(lb.buf)-1]
			}
			if len(lb.buf) > 0 {
				a.resetLineBufferTimer(lb)
			} else if lb.timer != nil {
				lb.timer.Stop()
				lb.timer = nil
			}

		default:
			if len(lb.buf) >= inputHistoryMaxInputLen {
				slog.Debug("[input-history] input truncated: max rune limit reached", "paneID", paneID)
				continue
			}
			lb.buf = append(lb.buf, r)
			a.resetLineBufferTimer(lb)
		}
	}

	a.inputLineBufMu.Unlock()

	// Write entries outside lock (defensive-coding-checklist: lock ordering).
	ts := time.Now().Format("20060102150405")
	for _, p := range toWrite {
		a.writeInputHistoryEntry(InputHistoryEntry{
			Timestamp: ts,
			PaneID:    paneID,
			Input:     p.text,
			Source:    p.source,
			Session:   p.session,
		})
	}
}

// resetLineBufferTimer resets (or creates) the inactivity flush timer for lb.
// Must be called with inputLineBufMu held.
// Each call increments lb.timerGen to prevent stale timer callbacks from
// writing to a buffer that has already been flushed or reassigned.
func (a *App) resetLineBufferTimer(lb *inputLineBuffer) {
	if lb.timer != nil {
		lb.timer.Stop()
	}
	lb.timerGen++
	gen := lb.timerGen
	paneID := lb.paneID
	lb.timer = time.AfterFunc(inputLineFlushTimeout, func() {
		if a.shuttingDown.Load() {
			return
		}
		a.flushLineBuffer(paneID, gen)
	})
}

// flushLineBuffer extracts and writes any pending buffered text for the given
// pane. Called by the inactivity timer or explicitly during shutdown.
//
// gen is the timer generation captured at timer creation. Stale callbacks
// (gen != lb.timerGen) are silently discarded to prevent double-writes when
// a new timer is started before an old one fires.
// During shutdown, gen=shutdownFlushSentinel is used to bypass the generation check.
func (a *App) flushLineBuffer(paneID string, gen uint64) {
	if gen != shutdownFlushSentinel && a.shuttingDown.Load() {
		// Ignore timer-based flush after shutdown has started.
		return
	}
	a.inputLineBufMu.Lock()
	lb, exists := a.inputLineBuffers[paneID]
	if !exists || lb == nil {
		a.inputLineBufMu.Unlock()
		return
	}
	// Discard stale timer callbacks: generation mismatch means a newer timer
	// has been started and will handle this pane's flush.
	// shutdownFlushSentinel bypasses generation check.
	if gen != shutdownFlushSentinel && lb.timerGen != gen {
		a.inputLineBufMu.Unlock()
		return
	}
	if gen == shutdownFlushSentinel && lb.timer != nil {
		lb.timer.Stop()
	}
	text := string(lb.buf)
	if text == "" {
		lb.timer = nil
		a.inputLineBufMu.Unlock()
		return
	}
	lb.buf = lb.buf[:0]
	lb.timer = nil
	source := lb.source
	session := lb.session
	a.inputLineBufMu.Unlock()

	a.writeInputHistoryEntry(InputHistoryEntry{
		Timestamp: time.Now().Format("20060102150405"),
		PaneID:    paneID,
		Input:     text,
		Source:    source,
		Session:   session,
	})
}

// flushAllLineBuffers stops all pending timers and writes any buffered text.
// Called during shutdown to persist incomplete lines before closing the file.
func (a *App) flushAllLineBuffers() {
	a.inputLineBufMu.Lock()
	if a.inputLineBuffers == nil {
		a.inputLineBufMu.Unlock()
		return
	}

	type pendingEntry struct {
		paneID  string
		text    string
		source  string
		session string
	}
	pending := make([]pendingEntry, 0, len(a.inputLineBuffers))
	for paneID, lb := range a.inputLineBuffers {
		if lb == nil {
			continue
		}
		if lb.timer != nil {
			lb.timer.Stop()
			lb.timer = nil
		}
		if text := string(lb.buf); text != "" {
			pending = append(pending, pendingEntry{paneID: paneID, text: text, source: lb.source, session: lb.session})
		}
	}
	a.inputLineBuffers = nil
	a.inputLineBufMu.Unlock()

	ts := time.Now().Format("20060102150405")
	for _, p := range pending {
		a.writeInputHistoryEntry(InputHistoryEntry{
			Timestamp: ts,
			PaneID:    p.paneID,
			Input:     p.text,
			Source:    p.source,
			Session:   p.session,
		})
	}
}

// writeInputHistoryEntry appends an entry to both the in-memory ring buffer and
// the JSONL file. All state mutations and the shouldEmit decision are made in a
// single lock acquisition.
//
// NOTE: The event model uses "ping + fetch" - the emitted event
// "app:input-history-updated" carries no payload. The frontend receives the ping
// and calls GetInputHistory() to obtain the full snapshot.
//
// NOTE: Unlike writeSessionLogEntry, Sync() is intentionally omitted here.
// Input history is high-frequency, non-critical data (keystrokes). The cost
// of fsync per batch outweighs the risk of losing a few entries on crash.
//
// NOTE: slog.Warn/Error must NOT be called while inputHistoryMu is held.
// The TeeHandler intercepts slog records and calls writeSessionLogEntry,
// which is independent, but to maintain the same safety discipline as
// writeSessionLogEntry we use fmt.Fprintf(os.Stderr, ...) for diagnostics.
func (a *App) writeInputHistoryEntry(entry InputHistoryEntry) {
	var marshalErr, writeErr error
	shouldEmit := false

	a.inputHistoryMu.Lock()

	// --- assign monotonic sequence number ---
	a.inputHistorySeq++
	entry.Seq = a.inputHistorySeq

	// --- file persistence ---
	if a.inputHistoryFile != nil {
		raw, err := json.Marshal(entry)
		if err != nil {
			marshalErr = err
		} else {
			raw = append(raw, '\n')
			if _, err := a.inputHistoryFile.Write(raw); err != nil {
				writeErr = err
			}
		}
	}

	// --- in-memory ring buffer append ---
	a.inputHistoryEntries.push(entry)

	// --- throttle decision ---
	now := time.Now()
	if now.Sub(a.inputHistoryLastEmit) >= inputHistoryEmitMinInterval {
		a.inputHistoryLastEmit = now
		shouldEmit = true
	}

	a.inputHistoryMu.Unlock()

	// Log internal errors via stderr to avoid TeeHandler recursion risk.
	if marshalErr != nil {
		fmt.Fprintf(os.Stderr, "[input-history] failed to marshal entry: %v\n", marshalErr)
	}
	if writeErr != nil {
		fmt.Fprintf(os.Stderr, "[input-history] failed to write entry: %v\n", writeErr)
	}

	// NOTE: Emit lightweight ping outside lock. nil payload is intentional -
	// the event is a notification trigger, not a data carrier.
	if shouldEmit {
		a.emitRuntimeEvent("app:input-history-updated", nil)
	}
}

// closeInputHistory flushes and closes the input history file handle.
func (a *App) closeInputHistory() {
	var closeErr error

	a.inputHistoryMu.Lock()
	if a.inputHistoryFile != nil {
		closeErr = a.inputHistoryFile.Close()
		a.inputHistoryFile = nil
	}
	a.inputHistoryMu.Unlock()

	if closeErr != nil {
		fmt.Fprintf(os.Stderr, "[input-history] failed to close history file: %v\n", closeErr)
	}
}

// GetInputHistory returns a copy of all in-memory input history entries.
// Wails-bound: called from the frontend to display the input history panel.
// Uses RLock because this is a read-only operation on the ring buffer.
func (a *App) GetInputHistory() []InputHistoryEntry {
	a.inputHistoryMu.RLock()
	defer a.inputHistoryMu.RUnlock()

	return a.inputHistoryEntries.snapshot()
}

// GetInputHistoryFilePath returns the absolute path to the current session's
// JSONL input history file.
// Wails-bound: used by the frontend for "open history file" actions.
// Uses RLock because this is a read-only operation.
func (a *App) GetInputHistoryFilePath() string {
	a.inputHistoryMu.RLock()
	defer a.inputHistoryMu.RUnlock()
	return a.inputHistoryPath
}
