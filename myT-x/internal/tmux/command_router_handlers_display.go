package tmux

import (
	"fmt"
	"log/slog"
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
	return okResp(expandFormat(format, target) + "\n")
}

func (r *CommandRouter) resolveTargetFromRequest(req ipc.TmuxRequest) (*TmuxPane, error) {
	target := mustString(req.Flags["-t"])
	callerPaneID := ParseCallerPane(req.CallerPane)
	return r.sessions.ResolveTarget(target, callerPaneID)
}

func (r *CommandRouter) resolveDirectionalPane(req ipc.TmuxRequest) (*TmuxPane, error) {
	current, err := r.sessions.ResolveTarget("", ParseCallerPane(req.CallerPane))
	if err != nil {
		return nil, err
	}

	// Capture stable identifiers before the implicit RLock from ResolveTarget is released.
	currentID := current.ID
	currentIDStr := current.IDString()

	panes, err := r.sessions.ListPanesByWindowTarget(currentIDStr, currentID, false)
	if err != nil {
		return nil, err
	}
	if len(panes) == 0 {
		return nil, fmt.Errorf("window has no panes")
	}

	// Find current pane's position within the fetched pane list (same lock scope)
	// instead of using current.Index which may be stale after RLock release (TOCTOU fix).
	idx := -1
	for i, p := range panes {
		if p != nil && p.ID == currentID {
			idx = i
			break
		}
	}
	if idx < 0 {
		// Current pane was removed between the two lock scopes; fall back to first pane.
		slog.Debug("[DEBUG-SELECT] current pane not found in window panes, using first pane",
			"paneID", currentIDStr)
		idx = 0
	}

	switch {
	case mustBool(req.Flags["-L"]), mustBool(req.Flags["-U"]):
		idx--
	case mustBool(req.Flags["-R"]), mustBool(req.Flags["-D"]):
		idx++
	default:
		return current, nil
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= len(panes) {
		idx = len(panes) - 1
	}
	candidate := panes[idx]
	if candidate == nil {
		return nil, fmt.Errorf("pane not found at directional index")
	}
	return r.sessions.ResolveTarget(candidate.IDString(), currentID)
}
