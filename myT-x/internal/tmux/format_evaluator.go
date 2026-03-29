// format_evaluator.go — Format evaluation: comparison expressions, filter evaluation, TOCTOU-safe expansion.
package tmux

import (
	"log/slog"
	"strings"
)

// evaluateComparisonExpr handles ==:a,b and !=:a,b comparison expressions.
// Both operands are expanded for nested #{var} references before comparison.
// Returns ("1"/"0", true) if the expression is a comparison, or ("", false) otherwise.
func evaluateComparisonExpr(expr string, pane *TmuxPane) (string, bool) {
	var op string
	var rest string
	if strings.HasPrefix(expr, "==:") {
		op = "=="
		rest = expr[3:]
	} else if strings.HasPrefix(expr, "!=:") {
		op = "!="
		rest = expr[3:]
	} else {
		return "", false
	}

	// Split on first comma to get lhs,rhs.
	commaIdx := findTopLevelComma(rest)
	if commaIdx < 0 {
		slog.Debug("[DEBUG-FORMAT] malformed comparison expr: missing comma", "op", op, "expr", expr)
		return "", false
	}
	lhs := rest[:commaIdx]
	rhs := rest[commaIdx+1:]

	// Expand nested #{var} references in both operands.
	lhs = expandFormat(lhs, pane)
	rhs = expandFormat(rhs, pane)

	switch op {
	case "==":
		if lhs == rhs {
			return "1", true
		}
		return "0", true
	case "!=":
		if lhs != rhs {
			return "1", true
		}
		return "0", true
	default:
		return "", false
	}
}

// findTopLevelComma finds the first comma that is not inside nested #{...}.
func findTopLevelComma(s string) int {
	depth := 0
	for i := 0; i < len(s); i++ {
		if i+1 < len(s) && s[i] == '#' && s[i+1] == '{' {
			depth++
			i++ // skip '{'
			continue
		}
		if s[i] == '}' && depth > 0 {
			depth--
			continue
		}
		if s[i] == ',' && depth == 0 {
			return i
		}
	}
	return -1
}

// expandFormatSafe resolves tmux format placeholders using a TOCTOU-safe clone.
//
// NOTE (I-03 TOCTOU-safe pattern): resolveTargetFromRequest returns a live
// *TmuxPane pointer whose Window.Session chain can be concurrently mutated
// after the lock is released. Passing that live pointer to expandFormat would
// dereference the chain outside any lock, creating a data race.
//
// This helper eliminates the race by:
//  1. Reading the pane's session name from GetPaneContextSnapshot (RLock-safe).
//  2. Obtaining a deep-cloned session via GetSession.
//  3. Locating the pane within the clone by ID.
//  4. Calling expandFormat on the fully-cloned pane, which has a safe
//     Window->Session chain that no concurrent goroutine can mutate.
//
// NOTE: A micro-TOCTOU gap exists between the two RLock acquisitions
// (GetPaneContextSnapshot and GetSession). If the session is renamed or
// removed between the two calls, the second lookup may fail even though
// the pane was valid during the first. This is accepted because:
//   - The fallback (nil pane) produces safe, empty-valued output.
//   - Holding a single lock across both lookups would require exposing
//     internal clone logic, adding complexity for a benign edge case.
//
// If any step fails (pane killed concurrently, session removed, etc.),
// a best-effort fallback using a bare TmuxPane with only scalar fields
// is returned. This keeps the tmux-shim contract of forwarding over aborting.
func expandFormatSafe(format string, paneID int, sessions *SessionManager) string {
	ctx, ctxErr := sessions.GetPaneContextSnapshot(paneID)
	if ctxErr != nil {
		slog.Debug("[DEBUG-FORMAT] expandFormatSafe: snapshot failed, using bare pane fallback",
			"paneId", paneID, "error", ctxErr)
		// DESIGN: Fallback uses nil pane intentionally. Partial information
		// (e.g., session name from a half-failed snapshot) is not injected
		// because the snapshot fields may already be stale by this point.
		// A nil pane produces well-defined empty/zero output for all format
		// variables, which is safer than mixing stale and missing values.
		return expandFormat(format, nil)
	}

	clonedSession, ok := sessions.GetSession(ctx.SessionName)
	if !ok {
		slog.Debug("[DEBUG-FORMAT] expandFormatSafe: session not found, using bare pane fallback",
			"paneId", paneID, "session", ctx.SessionName)
		return expandFormat(format, nil)
	}

	clonedPane := findPaneInSession(clonedSession, paneID)
	if clonedPane == nil {
		slog.Debug("[DEBUG-FORMAT] expandFormatSafe: pane not found in cloned session, using bare pane fallback",
			"paneId", paneID, "session", ctx.SessionName)
		return expandFormat(format, nil)
	}

	return expandFormat(format, clonedPane)
}

// findPaneInSession locates a pane by ID within a (cloned) session.
func findPaneInSession(session *TmuxSession, paneID int) *TmuxPane {
	if session == nil {
		return nil
	}
	for _, window := range session.Windows {
		if window == nil {
			continue
		}
		for _, pane := range window.Panes {
			if pane != nil && pane.ID == paneID {
				return pane
			}
		}
	}
	return nil
}

// evaluateFilter evaluates a tmux filter expression against a pane context.
// An empty or whitespace-only filter always returns true (include all items).
// The filter string is expanded via expandFormat, then checked for truthiness.
// A result of "" or "0" means the item is excluded (returns false).
// Unknown variables expand to "" which evaluates as false (item excluded).
func evaluateFilter(filter string, pane *TmuxPane) bool {
	if strings.TrimSpace(filter) == "" {
		return true
	}
	result := expandFormat(filter, pane)
	if result == "" || result == "0" {
		slog.Debug("[DEBUG-FILTER] pane excluded by filter",
			"filter", filter, "expanded", result)
		return false
	}
	return true
}

// evaluateFilterForSession evaluates a filter expression using session-level
// variables. An empty or whitespace-only filter always returns true (include all items).
// The active pane of the active window is used as context for variable expansion,
// matching tmux's behavior for list-sessions filtering.
func evaluateFilterForSession(filter string, session *TmuxSession) bool {
	if strings.TrimSpace(filter) == "" {
		return true
	}
	pane := firstPaneInSession(session)
	result := expandFormat(filter, pane)
	return result != "" && result != "0"
}
