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

func (r *CommandRouter) resolveTargetFromRequest(req ipc.TmuxRequest) (*TmuxPane, error) {
	target := mustString(req.Flags["-t"])
	callerPaneID := ParseCallerPane(req.CallerPane)
	return r.sessions.ResolveTarget(target, callerPaneID)
}

// resolveDirectionalPane resolves a pane in the direction specified by -L/-R/-U/-D flags.
// I-17: Delegates to SessionManager.ResolveDirectionalPane so that the current pane
// resolution, window pane listing, and directional navigation all occur under a single
// lock acquisition, eliminating the TOCTOU race of three independent lock scopes.
func (r *CommandRouter) resolveDirectionalPane(req ipc.TmuxRequest) (*TmuxPane, error) {
	callerPaneID := ParseCallerPane(req.CallerPane)

	var direction DirectionalPaneDirection
	switch {
	case mustBool(req.Flags["-L"]), mustBool(req.Flags["-U"]):
		direction = DirPrev
	case mustBool(req.Flags["-R"]), mustBool(req.Flags["-D"]):
		direction = DirNext
	default:
		direction = DirNone
	}

	return r.sessions.ResolveDirectionalPane(callerPaneID, direction)
}
