package tmux

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"myT-x/internal/ipc"
)

const maxLoadBufferFileSize = 10 * 1024 * 1024 // 10 MiB

type loadBufferReadCloser interface {
	io.Reader
	io.Closer
	Stat() (os.FileInfo, error)
}

type saveBufferWriteCloser interface {
	io.Writer
	io.Closer
}

// initBufferFileOps sets default buffer file I/O operations on the router
// if they are not already configured (e.g. by tests).
func (r *CommandRouter) initBufferFileOps() {
	if r.openLoadBufferFile == nil {
		r.openLoadBufferFile = func(path string) (loadBufferReadCloser, error) { return os.Open(path) }
	}
	if r.openSaveBufferFile == nil {
		r.openSaveBufferFile = func(path string, flag int, perm os.FileMode) (saveBufferWriteCloser, error) {
			return os.OpenFile(path, flag, perm)
		}
	}
	if r.removeSaveBufferFile == nil {
		r.removeSaveBufferFile = os.Remove
	}
}

// handleListBuffers lists all paste buffers with optional format.
func (r *CommandRouter) handleListBuffers(req ipc.TmuxRequest) ipc.TmuxResponse {
	format := mustString(req.Flags["-F"])
	buffers := r.buffers.List()
	slog.Debug("[DEBUG-BUFFER] list-buffers", "count", len(buffers), "hasFormat", format != "")

	if len(buffers) == 0 {
		return okResp("")
	}

	lines := make([]string, 0, len(buffers))
	for _, buf := range buffers {
		lines = append(lines, formatBufferLine(buf, format))
	}
	return okResp(joinLines(lines))
}

// handleSetBuffer creates, updates, appends to, or renames a paste buffer.
// Flags: -b (buffer name), -a (append), -n (rename).
// Data is taken from req.Args[0].
func (r *CommandRouter) handleSetBuffer(req ipc.TmuxRequest) ipc.TmuxResponse {
	bufferName := mustString(req.Flags["-b"])
	appendMode := mustBool(req.Flags["-a"])
	newName := mustString(req.Flags["-n"])

	// -n: rename operation (no data needed).
	if newName != "" {
		if bufferName == "" {
			return errResp(fmt.Errorf("set-buffer -n requires -b to specify the buffer to rename"))
		}
		if err := r.buffers.Rename(bufferName, newName); err != nil {
			return errResp(err)
		}
		slog.Debug("[DEBUG-BUFFER] set-buffer: renamed", "from", bufferName, "to", newName)
		return okResp("")
	}

	// Data operation: create or update.
	if len(req.Args) == 0 {
		return errResp(fmt.Errorf("set-buffer requires data argument"))
	}
	data := []byte(req.Args[0])
	r.buffers.Set(bufferName, data, appendMode)

	slog.Debug("[DEBUG-BUFFER] set-buffer: data set",
		"buffer", bufferName,
		"append", appendMode,
		"dataLen", len(data),
	)
	return okResp("")
}

// handlePasteBuffer pastes buffer contents to a target pane's terminal.
// Flags: -b (buffer name), -t (target pane), -d (delete after paste),
// -p (bracket paste mode), -r (in myT-x, replaces LF with CR), -s (separator).
func (r *CommandRouter) handlePasteBuffer(req ipc.TmuxRequest) ipc.TmuxResponse {
	bufferName := mustString(req.Flags["-b"])
	deleteAfter := mustBool(req.Flags["-d"])
	bracketPaste := mustBool(req.Flags["-p"])
	replaceNewlines := mustBool(req.Flags["-r"])
	separator := mustString(req.Flags["-s"])

	// Resolve the buffer to paste.
	var buf *PasteBuffer
	var ok bool
	if bufferName != "" {
		buf, ok = r.buffers.Get(bufferName)
	} else {
		buf, ok = r.buffers.Latest()
	}
	if !ok {
		return errResp(fmt.Errorf("no buffer found"))
	}

	// Resolve target pane.
	target, err := r.resolveTargetFromRequest(req)
	if err != nil {
		return errResp(err)
	}
	if target.Terminal == nil {
		return errResp(fmt.Errorf("pane has no terminal: %s", target.IDString()))
	}

	// Prepare paste data.
	// tmux semantics: -s (separator) takes priority over -r (replace newlines).
	// When both are specified, only -s is applied.
	data := buf.Data
	if separator != "" {
		data = []byte(strings.ReplaceAll(string(data), "\n", separator))
	} else if replaceNewlines {
		data = []byte(strings.ReplaceAll(string(data), "\n", "\r"))
	}

	// Bracket paste mode: wrap data with escape sequences.
	if bracketPaste {
		bracketStart := []byte("\033[200~")
		bracketEnd := []byte("\033[201~")
		wrapped := make([]byte, 0, len(bracketStart)+len(data)+len(bracketEnd))
		wrapped = append(wrapped, bracketStart...)
		wrapped = append(wrapped, data...)
		wrapped = append(wrapped, bracketEnd...)
		data = wrapped
	}

	slog.Debug("[DEBUG-BUFFER] paste-buffer",
		"buffer", buf.Name,
		"targetPane", target.IDString(),
		"dataLen", len(data),
		"bracketPaste", bracketPaste,
	)

	if err := writeSendKeysPayload(target.Terminal, data); err != nil {
		return errResp(err)
	}

	// Delete buffer after successful paste if requested.
	if deleteAfter {
		effectiveName := bufferName
		if effectiveName == "" {
			effectiveName = buf.Name
		}
		r.buffers.Delete(effectiveName)
	}
	return okResp("")
}

// handleDeleteBuffer deletes a named paste buffer, or the latest buffer when
// -b is omitted.
func (r *CommandRouter) handleDeleteBuffer(req ipc.TmuxRequest) ipc.TmuxResponse {
	bufferName := mustString(req.Flags["-b"])
	if bufferName != "" {
		if !r.buffers.Delete(bufferName) {
			return errResp(fmt.Errorf("buffer not found: %s", bufferName))
		}
		slog.Debug("[DEBUG-BUFFER] delete-buffer", "buffer", bufferName)
		return okResp("")
	}

	deletedName, ok := r.buffers.DeleteLatest()
	if !ok {
		return errResp(fmt.Errorf("no buffer found"))
	}
	slog.Debug("[DEBUG-BUFFER] delete-buffer", "buffer", deletedName)
	return okResp("")
}

// handleLoadBuffer reads a file and stores its contents in a paste buffer.
// Flags: -b (buffer name), -w (no-op: clipboard), -t (no-op: target client).
func (r *CommandRouter) handleLoadBuffer(req ipc.TmuxRequest) ipc.TmuxResponse {
	// Require exactly one positional arg (file path).
	if len(req.Args) == 0 {
		return errResp(fmt.Errorf("load-buffer requires a file path argument"))
	}
	path := req.Args[0]

	// Reject stdin "-" (not supported in myT-x).
	if path == "-" {
		return errResp(fmt.Errorf("load-buffer from stdin is not supported"))
	}

	file, err := r.openLoadBufferFile(path)
	if err != nil {
		return errResp(fmt.Errorf("load-buffer: %w", err))
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			slog.Warn("[WARN-BUFFER] load-buffer: failed to close file", "path", path, "error", closeErr)
		}
	}()

	info, err := file.Stat()
	if err != nil {
		return errResp(fmt.Errorf("load-buffer: %w", err))
	}
	if info.Size() > maxLoadBufferFileSize {
		return errResp(fmt.Errorf("load-buffer: file too large (%d bytes, max %d)", info.Size(), maxLoadBufferFileSize))
	}

	// Read from the already-open file handle and enforce the size limit again
	// in case the file grows after Stat but before the read completes.
	data, err := io.ReadAll(io.LimitReader(file, maxLoadBufferFileSize+1))
	if err != nil {
		return errResp(fmt.Errorf("load-buffer: %w", err))
	}
	if len(data) > maxLoadBufferFileSize {
		return errResp(fmt.Errorf("load-buffer: file too large (%d bytes, max %d)", len(data), maxLoadBufferFileSize))
	}

	// Empty file: silently succeed (matches tmux behavior).
	if len(data) == 0 {
		slog.Debug("[DEBUG-BUFFER] load-buffer: empty file, nothing to store", "path", path)
		return okResp("")
	}

	// Store in buffer.
	bufferName := mustString(req.Flags["-b"])
	r.buffers.Set(bufferName, data, false)

	slog.Debug("[DEBUG-BUFFER] load-buffer: loaded", "path", path, "buffer", bufferName, "size", len(data))
	return okResp("")
}

// handleSaveBuffer writes a paste buffer to a file or stdout.
// Flags: -a (append), -b (buffer name).
func (r *CommandRouter) handleSaveBuffer(req ipc.TmuxRequest) ipc.TmuxResponse {
	if len(req.Args) == 0 {
		return errResp(fmt.Errorf("save-buffer requires a file path argument"))
	}
	path := req.Args[0]
	appendMode := mustBool(req.Flags["-a"])
	bufferName := mustString(req.Flags["-b"])

	var (
		buf *PasteBuffer
		ok  bool
	)
	if bufferName == "" {
		buf, ok = r.buffers.Latest()
		if !ok {
			return errResp(fmt.Errorf("no buffers"))
		}
	} else {
		buf, ok = r.buffers.Get(bufferName)
		if !ok {
			return errResp(fmt.Errorf("no buffer %s", bufferName))
		}
	}

	if path == "-" {
		// Path "-" writes buffer data to stdout instead of a file.
		slog.Debug("[DEBUG-BUFFER] save-buffer: wrote to stdout", "buffer", buf.Name, "size", len(buf.Data))
		return okResp(string(buf.Data))
	}

	flags := os.O_CREATE | os.O_WRONLY
	if appendMode {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	file, err := r.openSaveBufferFile(path, flags, 0o644)
	if err != nil {
		return errResp(fmt.Errorf("save-buffer: %w", err))
	}
	if _, err := file.Write(buf.Data); err != nil {
		_ = file.Close()
		r.removePartialSaveBufferFile(path, appendMode)
		return errResp(fmt.Errorf("save-buffer: %w", err))
	}
	if err := file.Close(); err != nil {
		r.removePartialSaveBufferFile(path, appendMode)
		return errResp(fmt.Errorf("save-buffer: %w", err))
	}

	slog.Debug("[DEBUG-BUFFER] save-buffer: wrote file",
		"path", path,
		"buffer", buf.Name,
		"append", appendMode,
		"size", len(buf.Data),
	)
	return okResp("")
}

func (r *CommandRouter) removePartialSaveBufferFile(path string, appendMode bool) {
	if appendMode || strings.TrimSpace(path) == "" {
		return
	}
	if err := r.removeSaveBufferFile(path); err != nil && !os.IsNotExist(err) {
		slog.Warn("[WARN-BUFFER] save-buffer: failed to remove partial file", "path", path, "error", err)
	}
}

// handleCapturePane captures pane output and stores it in a buffer or prints to stdout.
// Flags: -p (print to stdout), -b (buffer name), -t (target pane), -q (quiet errors).
// No-op flags: -e, -J, -N, -T, -a, -C, -P, -M (raw output mode).
func (r *CommandRouter) handleCapturePane(req ipc.TmuxRequest) ipc.TmuxResponse {
	printToStdout := mustBool(req.Flags["-p"])
	bufferName := mustString(req.Flags["-b"])
	quiet := mustBool(req.Flags["-q"])

	// Resolve target pane.
	target, err := r.resolveTargetFromRequest(req)
	if err != nil {
		if quiet {
			return okResp("")
		}
		return errResp(err)
	}
	targetPaneID := target.IDString()

	// Get output history.
	historyRef := target.OutputHistory
	if historyRef == nil {
		if quiet {
			return okResp("")
		}
		return errResp(fmt.Errorf("pane has no output history: %s", targetPaneID))
	}

	data, err := selectCapturePaneLines(historyRef.Capture(), req.Flags["-S"], req.Flags["-E"])
	if err != nil {
		// tmux-shim policy (CLAUDE.md §tmux-shim について): on -q, parse errors
		// are swallowed and empty output is returned — errors go to log only.
		// DO NOT convert this into an error response — it will break tmux-shim
		// transparency contracts. See /ACCEPTED_DESIGN_DECISIONS.md AD-003.
		if quiet {
			slog.Debug("[DEBUG-BUFFER] capture-pane: parse failed, quiet swallow", "pane", targetPaneID, "err", err)
			return okResp("")
		}
		return errResp(err)
	}

	if printToStdout {
		slog.Debug("[DEBUG-BUFFER] capture-pane: print to stdout", "pane", targetPaneID, "size", len(data))
		return okResp(string(data))
	}

	// Store in paste buffer.
	r.buffers.Set(bufferName, data, false)
	slog.Debug("[DEBUG-BUFFER] capture-pane: stored in buffer", "pane", targetPaneID, "buffer", bufferName, "size", len(data))
	return okResp("")
}

type capturePaneLineSpan struct {
	start int
	end   int
}

func selectCapturePaneLines(data []byte, startFlag any, endFlag any) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}

	lineSpans := capturePaneLineSpans(data)
	if len(lineSpans) == 0 {
		return nil, nil
	}

	startIndex, err := resolveCapturePaneLineIndex(startFlag, len(lineSpans), true)
	if err != nil {
		return nil, err
	}
	endIndex, err := resolveCapturePaneLineIndex(endFlag, len(lineSpans), false)
	if err != nil {
		return nil, err
	}
	if startIndex > endIndex {
		return nil, nil
	}

	selected := data[lineSpans[startIndex].start:lineSpans[endIndex].end]
	return append([]byte(nil), selected...), nil
}

func capturePaneLineSpans(data []byte) []capturePaneLineSpan {
	lineCount := bytes.Count(data, []byte{'\n'})
	if len(data) > 0 && data[len(data)-1] != '\n' {
		lineCount++
	}
	spans := make([]capturePaneLineSpan, 0, lineCount)
	lineStart := 0
	for idx, ch := range data {
		if ch != '\n' {
			continue
		}
		spans = append(spans, capturePaneLineSpan{start: lineStart, end: idx + 1})
		lineStart = idx + 1
	}
	if lineStart < len(data) {
		spans = append(spans, capturePaneLineSpan{start: lineStart, end: len(data)})
	}
	return spans
}

func resolveCapturePaneLineIndex(flag any, lineCount int, isStart bool) (int, error) {
	if lineCount <= 0 {
		return 0, nil
	}

	defaultIndex := 0
	label := "start"
	if !isStart {
		defaultIndex = lineCount - 1
		label = "end"
	}

	text := strings.TrimSpace(mustString(flag))
	if text == "" || text == "-" {
		return defaultIndex, nil
	}

	value, err := strconv.Atoi(text)
	if err != nil {
		return 0, fmt.Errorf("invalid %s line for capture-pane: %q", label, text)
	}

	if value < 0 {
		value = lineCount + value
	}
	if value < 0 {
		return 0, nil
	}
	if value >= lineCount {
		return lineCount - 1, nil
	}
	return value, nil
}
