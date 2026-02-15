package terminal

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"sync"
)

// PID returns the process id.
func (t *Terminal) PID() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.pty != nil {
		return t.pty.Pid()
	}
	if t.cmd == nil || t.cmd.Process == nil {
		return 0
	}
	return t.cmd.Process.Pid
}

// IsClosed reports whether Close has been called.
func (t *Terminal) IsClosed() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.closed
}

// Write writes input bytes to the PTY.
func (t *Terminal) Write(data []byte) (int, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed {
		slog.Warn("[terminal] Write: terminal is closed", "dataLen", len(data))
		return 0, errors.New("terminal closed")
	}
	if t.pty != nil {
		n, err := t.pty.Write(data)
		if err != nil {
			slog.Warn("[terminal] Write failed", "error", err, "dataLen", len(data))
		}
		return n, err
	}
	if t.ptmx != nil {
		n, err := t.ptmx.Write(data)
		if err != nil {
			slog.Warn("[terminal] Write (ptmx) failed", "error", err, "dataLen", len(data))
		}
		return n, err
	}
	if t.stdin == nil {
		return 0, errors.New("terminal stdin unavailable")
	}
	payload := normalizePipeInput(data)
	n, err := t.stdin.Write(payload)
	if err != nil {
		slog.Warn("[terminal] Write (stdin) failed", "error", err, "dataLen", len(data))
	}
	return n, err
}

// Resize updates the PTY window size.
func (t *Terminal) Resize(cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return errors.New("invalid size")
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed {
		return errors.New("terminal closed")
	}
	if t.pty != nil {
		return t.pty.Resize(cols, rows)
	}
	if t.ptmx != nil {
		return resizePtmx(t.ptmx, cols, rows)
	}
	// Pipe-mode fallback has no PTY resize.
	return nil
}

// ReadLoop continuously reads terminal output until closed.
func (t *Terminal) ReadLoop(onData func([]byte)) {
	if onData == nil {
		return
	}
	t.mu.RLock()
	ptyImpl := t.pty
	file := t.ptmx
	stdout := t.stdout
	stderr := t.stderr
	t.mu.RUnlock()

	if ptyImpl != nil {
		slog.Info("[terminal] ReadLoop: using ConPTY backend")
		readSource(ptyImpl, onData)
		slog.Info("[terminal] ReadLoop: ConPTY readSource returned (loop ended)")
		return
	}
	if file != nil {
		slog.Info("[terminal] ReadLoop: using Unix PTY backend")
		readSource(file, onData)
		slog.Info("[terminal] ReadLoop: Unix PTY readSource returned (loop ended)")
		return
	}

	slog.Info("[terminal] ReadLoop: using pipe mode")
	var wg sync.WaitGroup
	if stdout != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			readSource(stdout, onData)
		}()
	}
	if stderr != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			readSource(stderr, onData)
		}()
	}
	wg.Wait()
	slog.Info("[terminal] ReadLoop: pipe mode ended")
}

func readSource(reader io.Reader, onData func([]byte)) {
	slog.Info("[terminal] readSource started")
	buf := make([]byte, 32*1024)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			// onData must consume the bytes during this call because the backing
			// buffer is reused on the next read.
			onData(buf[:n])
		}
		if err != nil {
			slog.Warn("[terminal] readSource exiting", "error", err, "bytesInLastRead", n)
			return
		}
	}
}

// normalizePipeInput adapts CR-only input into CRLF for pipe-mode shells on Windows.
// ConPTY and PTY backends bypass this path.
func normalizePipeInput(data []byte) []byte {
	if len(data) == 0 {
		return data
	}

	hasCR := false
	for _, b := range data {
		if b == '\r' {
			hasCR = true
			break
		}
	}
	if !hasCR {
		return data
	}

	out := make([]byte, 0, len(data)+8)
	for i := 0; i < len(data); i++ {
		b := data[i]
		if b != '\r' {
			out = append(out, b)
			continue
		}

		out = append(out, '\r')
		if i+1 >= len(data) || data[i+1] != '\n' {
			out = append(out, '\n')
		}
	}
	return out
}

// Close closes PTY and terminates process.
func (t *Terminal) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return t.closeErr
	}
	t.closed = true

	var firstErr error
	if t.pty != nil {
		if err := t.pty.Close(); err != nil {
			firstErr = err
		}
	}
	if t.cmd != nil && t.cmd.Process != nil {
		if killErr := t.cmd.Process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
			slog.Debug("[terminal] process kill during close failed", "error", killErr)
		}
	}
	if t.stdin != nil {
		if err := t.stdin.Close(); err != nil && firstErr == nil {
			slog.Warn("[terminal] close stdin failed", "error", err)
			firstErr = err
		}
	}
	if t.stdout != nil {
		if err := t.stdout.Close(); err != nil && firstErr == nil {
			slog.Warn("[terminal] close stdout failed", "error", err)
			firstErr = err
		}
	}
	if t.stderr != nil {
		if err := t.stderr.Close(); err != nil && firstErr == nil {
			slog.Warn("[terminal] close stderr failed", "error", err)
			firstErr = err
		}
	}
	if t.ptmx != nil {
		if err := t.ptmx.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	t.closeErr = firstErr
	return firstErr
}
