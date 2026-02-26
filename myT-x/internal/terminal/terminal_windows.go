//go:build windows

package terminal

import (
	"log/slog"
	"os"
	"strings"
	"syscall"
)

// Start launches a PTY process using ConPTY on Windows.
// Falls back to pipe mode if ConPTY is not available.
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

	// NOTE: ConPTY manages its own console window via CreateProcess with
	// EXTENDED_STARTUPINFO_PRESENT; HideWindow is not needed for that path.
	// Only the pipe-mode fallback (startPipeMode) requires HideWindow.
	if shouldUseConPty() && IsConPtyAvailable() {
		cmdLine := buildCommandLine(cfg.Shell, cfg.Args)
		opts := []ConPtyOption{
			ConPtyDimensions(cfg.Columns, cfg.Rows),
		}
		if cfg.Dir != "" {
			opts = append(opts, ConPtyWorkDir(cfg.Dir))
		}
		if len(cfg.Env) > 0 {
			opts = append(opts, ConPtyEnv(cfg.Env))
		}
		cpty, err := startConPty(cmdLine, opts...)
		if err == nil {
			if _, err := cpty.Write([]byte("chcp 65001\r\n")); err != nil {
				slog.Warn("failed to set UTF-8 code page", "error", err)
			}
			return &Terminal{pty: cpty}, nil
		}
		// ConPTY was available but failed to start; log for debugging and fall through to pipe mode.
		slog.Warn("[WARN-TERMINAL] ConPTY start failed, falling back to pipe mode", "error", err)
	}

	return startPipeMode(cfg)
}

// shouldUseConPty decides whether to use ConPTY based on environment variables.
//
// Environment variable parsing policy (default = ConPTY enabled):
//   - GO_TMUX_DISABLE_CONPTY: opt-in disabling — only "1/true/yes/on" disables.
//     Any other value (including "0", "random") does NOT disable.
//   - GO_TMUX_ENABLE_CONPTY: only "0/false/no/off" explicitly disables.
//     Any other value (including unset, empty, unrecognized) keeps ConPTY enabled.
//
// This asymmetry is intentional: both variables default toward "enabled" so
// that unrecognized values never accidentally disable ConPTY.
func shouldUseConPty() bool {
	disable := strings.TrimSpace(strings.ToLower(os.Getenv("GO_TMUX_DISABLE_CONPTY")))
	switch disable {
	case "1", "true", "yes", "on":
		return false
	}

	value := strings.TrimSpace(strings.ToLower(os.Getenv("GO_TMUX_ENABLE_CONPTY")))
	// Use opt-out approach (matching the DISABLE switch above) so that any
	// unrecognized value (including empty) defaults to enabled, consistent with ConPTY-on-by-default.
	switch value {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

func defaultShell() string {
	return "powershell.exe"
}

// buildCommandLine constructs a properly escaped command line string.
// ConPTY's CreateProcess takes a single command line string.
// Uses syscall.EscapeArg to handle paths with spaces.
func buildCommandLine(shell string, args []string) string {
	if len(args) == 0 {
		return syscall.EscapeArg(shell)
	}
	parts := make([]string, 0, 1+len(args))
	parts = append(parts, syscall.EscapeArg(shell))
	for _, arg := range args {
		parts = append(parts, syscall.EscapeArg(arg))
	}
	return strings.Join(parts, " ")
}
