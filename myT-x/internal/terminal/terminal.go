package terminal

import (
	"io"
	"os"
	"os/exec"
	"sync"

	"myT-x/internal/procutil"
)

const (
	defaultCols = 120
	defaultRows = 40
)

// Config configures a terminal process.
type Config struct {
	Shell   string
	Args    []string
	Dir     string
	Env     []string
	Columns int
	Rows    int
}

// ptyReadWriteCloser abstracts a PTY backend that supports
// Read, Write, Resize, Close, and Pid.
// ConPty on Windows implements this interface.
type ptyReadWriteCloser interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Resize(width, height int) error
	Close() error
	Pid() int
}

// Terminal wraps one PTY process.
type Terminal struct {
	mu       sync.RWMutex
	cmd      *exec.Cmd          // non-nil for Unix/pipe mode, nil for ConPTY mode
	ptmx     *os.File           // Unix PTY master (creack/pty)
	pty      ptyReadWriteCloser // ConPTY on Windows; nil on Unix/pipe
	stdin    io.WriteCloser     // pipe fallback
	stdout   io.ReadCloser      // pipe fallback
	stderr   io.ReadCloser      // pipe fallback
	closed   bool
	closeErr error
}

// startPipeMode starts a process in pipe mode as fallback.
// SECURITY: cfg.Shell and cfg.Args are trusted values from internal Config struct,
// populated by application code (not user input).
func startPipeMode(cfg Config) (*Terminal, error) {
	cmd := exec.Command(cfg.Shell, cfg.Args...)
	cmd.Dir = cfg.Dir
	if len(cfg.Env) > 0 {
		cmd.Env = cfg.Env
	}
	procutil.HideWindow(cmd)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return nil, err
	}
	return &Terminal{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}, nil
}
