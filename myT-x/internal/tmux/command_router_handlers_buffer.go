package tmux

import (
	"fmt"
	"io"
	"log/slog"
	"os"
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

var openLoadBufferFile = func(path string) (loadBufferReadCloser, error) {
	return os.Open(path)
}

var openSaveBufferFile = func(path string, flag int, perm os.FileMode) (saveBufferWriteCloser, error) {
	return os.OpenFile(path, flag, perm)
}

var removeSaveBufferFile = os.Remove

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

	file, err := openLoadBufferFile(path)
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

	file, err := openSaveBufferFile(path, flags, 0o644)
	if err != nil {
		return errResp(fmt.Errorf("save-buffer: %w", err))
	}
	if _, err := file.Write(buf.Data); err != nil {
		_ = file.Close()
		removePartialSaveBufferFile(path, appendMode)
		return errResp(fmt.Errorf("save-buffer: %w", err))
	}
	if err := file.Close(); err != nil {
		removePartialSaveBufferFile(path, appendMode)
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

func removePartialSaveBufferFile(path string, appendMode bool) {
	if appendMode || strings.TrimSpace(path) == "" {
		return
	}
	if err := removeSaveBufferFile(path); err != nil && !os.IsNotExist(err) {
		slog.Warn("[WARN-BUFFER] save-buffer: failed to remove partial file", "path", path, "error", err)
	}
}

// handleCapturePane captures pane output and stores it in a buffer or prints to stdout.
// Flags: -p (print to stdout), -b (buffer name), -t (target pane), -q (quiet errors).
// No-op flags: -e, -J, -N, -T, -a, -C, -P, -M, -S, -E (raw output mode).
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

	data := historyRef.Capture()
	// Compatibility deviation: myT-x currently ignores -S/-E and always returns
	// the full retained history for capture-pane.
	if _, hasStartRange := req.Flags["-S"]; hasStartRange {
		slog.Debug("[DEBUG-BUFFER] capture-pane: -S flag ignored (full history returned)", "pane", targetPaneID)
	}
	if _, hasEndRange := req.Flags["-E"]; hasEndRange {
		slog.Debug("[DEBUG-BUFFER] capture-pane: -E flag ignored (full history returned)", "pane", targetPaneID)
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
