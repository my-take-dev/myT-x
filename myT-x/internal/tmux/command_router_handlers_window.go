package tmux

import (
	"log/slog"

	"myT-x/internal/ipc"
)

// handleActivateWindow signals the host application to bring its window
// to the foreground. Used by the second instance to activate the first
// instance's window before exiting.
func (r *CommandRouter) handleActivateWindow(_ ipc.TmuxRequest) ipc.TmuxResponse {
	slog.Debug("[DEBUG-IPC] activate-window command received")
	r.emitter.Emit("app:activate-window", nil)
	return ipc.TmuxResponse{ExitCode: 0, Stdout: "ok\n"}
}
