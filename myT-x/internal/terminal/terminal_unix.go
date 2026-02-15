//go:build !windows

package terminal

import (
	"errors"
	"os"
	"os/exec"

	"github.com/creack/pty"
)

// Start launches a PTY process using creack/pty.
// Falls back to pipe mode if PTY is not available.
func Start(cfg Config) (*Terminal, error) {
	if cfg.Shell == "" {
		cfg.Shell = defaultShell()
	}
	if cfg.Columns <= 0 {
		cfg.Columns = defaultCols
	}
	if cfg.Rows <= 0 {
		cfg.Rows = defaultRows
	}

	// SECURITY: cfg.Shell and cfg.Args are trusted values from internal Config struct,
	// populated by application code (not user input).
	cmd := exec.Command(cfg.Shell, cfg.Args...)
	cmd.Dir = cfg.Dir
	if len(cfg.Env) > 0 {
		cmd.Env = cfg.Env
	}

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Cols: uint16(cfg.Columns),
		Rows: uint16(cfg.Rows),
	})
	if err == nil {
		return &Terminal{
			cmd:  cmd,
			ptmx: ptmx,
		}, nil
	}
	if !errors.Is(err, pty.ErrUnsupported) {
		return nil, err
	}

	return startPipeMode(cfg)
}

func defaultShell() string {
	if sh := os.Getenv("SHELL"); sh != "" {
		return sh
	}
	return "/bin/sh"
}
