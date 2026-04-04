package tmux

import (
	"log/slog"

	"myT-x/internal/ipc"
)

func (r *CommandRouter) handleSetOption(req ipc.TmuxRequest) ipc.TmuxResponse {
	slog.Debug("[DEBUG-OPTION] set-option ignored (tmux compatibility no-op)",
		"flags", req.Flags,
		"args", req.Args,
	)
	return okResp("")
}

func (r *CommandRouter) handleSelectLayout(req ipc.TmuxRequest) ipc.TmuxResponse {
	slog.Debug("[DEBUG-OPTION] select-layout ignored (tmux compatibility no-op)",
		"flags", req.Flags,
		"args", req.Args,
	)
	return okResp("")
}
