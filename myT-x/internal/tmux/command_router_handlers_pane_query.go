// command_router_handlers_pane_query.go — Pane query handler: list-panes.
package tmux

import (
	"log/slog"

	"myT-x/internal/ipc"
)

func (r *CommandRouter) handleListPanes(req ipc.TmuxRequest) ipc.TmuxResponse {
	target := mustString(req.Flags["-t"])
	allSessions := mustBool(req.Flags["-a"])
	allInSession := mustBool(req.Flags["-s"])
	format := mustString(req.Flags["-F"])
	filter := mustString(req.Flags["-f"])
	callerPaneID := ParseCallerPane(req.CallerPane)

	// Session-scoped filtering: when the caller pane has MYTX_SESSION set,
	// only include panes from the matching session. This provides automatic
	// session isolation for MCP tools (Claude Code, etc.).
	var callerMytxSession string
	if callerPaneID >= 0 {
		if callerCtx, ctxErr := r.sessions.GetPaneContextSnapshot(callerPaneID); ctxErr == nil {
			callerMytxSession = callerCtx.Env["MYTX_SESSION"]
		}
	}
	if callerMytxSession != "" {
		slog.Debug("[DEBUG-LISTPANES] session filtering applied",
			"callerPaneID", callerPaneID,
			"callerMytxSession", callerMytxSession,
		)
	}

	// -a: list panes across all sessions (highest precedence).
	// Pattern follows handleListWindows: ListSessions returns deep clones with
	// intact Window->Session back-references, enabling format variable expansion
	// for session_name etc. outside lock scope.
	if allSessions {
		sessions := r.sessions.ListSessions()
		lines := make([]string, 0)
		for _, session := range sessions {
			if callerMytxSession != "" && session.Name != callerMytxSession {
				continue
			}
			for _, window := range session.Windows {
				if window == nil {
					continue
				}
				for _, pane := range window.Panes {
					if pane == nil {
						continue
					}
					if !evaluateFilter(filter, pane) {
						continue
					}
					lines = append(lines, formatPaneLine(pane, format))
				}
			}
		}
		return okResp(joinLines(lines))
	}

	panes, err := r.sessions.ListPanesByWindowTarget(target, callerPaneID, allInSession)
	if err != nil {
		return errResp(err)
	}
	// I-4: panes is []TmuxPane (value copies) to prevent data-race on internal
	// pointers. The slice is already in correct order (window-by-window, then
	// pane-by-pane within each window) from the collection loop in
	// ListPanesByWindowTarget. No re-sorting is needed; value-copied panes
	// have Window=nil, making the original multi-level sort impossible.
	//
	// NOTE: Value-copied panes have Window=nil, so filter expressions that
	// reference session/window variables will see empty/zero values.
	// This is a known limitation of the non-(-a) path.
	if filter != "" {
		slog.Warn("[WARN-LISTPANES] filter on non-(-a) path: session/window variables are unavailable (Window=nil in value copies); filter may exclude all items",
			"filter", filter, "target", target, "paneCount", len(panes))
	}

	lines := make([]string, 0, len(panes))
	for i := range panes {
		if !evaluateFilter(filter, &panes[i]) {
			continue
		}
		lines = append(lines, formatPaneLine(&panes[i], format))
	}
	return okResp(joinLines(lines))
}
