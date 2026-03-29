package tmux

import (
	"strings"

	"myT-x/internal/ipc"
)

func (r *CommandRouter) handleDisplayMessage(req ipc.TmuxRequest) ipc.TmuxResponse {
	if !mustBool(req.Flags["-p"]) {
		return okResp("")
	}

	target, err := r.resolveTargetFromRequest(req)
	if err != nil {
		return errResp(err)
	}

	var format string
	if len(req.Args) > 0 {
		format = strings.Join(req.Args, " ")
	}
	if strings.TrimSpace(format) == "" {
		return okResp("\n")
	}
	// NOTE (I-03 TOCTOU-safe pattern): Use expandFormatSafe instead of passing
	// the live target pointer to expandFormat. target.Window.Session is a live
	// pointer chain that can be concurrently mutated after lock release.
	// expandFormatSafe obtains a deep-cloned pane for safe traversal.
	return okResp(expandFormatSafe(format, target.ID, r.sessions) + "\n")
}
