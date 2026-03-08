package tmux

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

const (
	defaultSessionListFormat = "#{session_name}: #{session_windows} windows (created #{session_created_human})"
	defaultPaneListFormat    = "#{pane_index}: [#{pane_width}x#{pane_height}] [history 0/2000, 0 bytes] #{pane_id}#{pane_active_suffix}"
	defaultWindowListFormat  = "#{window_index}: #{window_name} (#{window_panes} panes)"
)

func formatSessionLine(session *TmuxSession, customFormat string) string {
	format := strings.TrimSpace(customFormat)
	if format == "" {
		format = defaultSessionListFormat
	}

	// All format variables — including session_created_human — are resolved
	// by lookupFormatVariable through expandFormat. No pre-expansion needed.
	return expandFormat(format, firstPaneInSession(session))
}

func formatWindowLine(window *TmuxWindow, customFormat string) string {
	format := strings.TrimSpace(customFormat)
	if format == "" {
		format = defaultWindowListFormat
	}
	var pane *TmuxPane
	if window != nil && len(window.Panes) > 0 {
		if window.ActivePN >= 0 && window.ActivePN < len(window.Panes) {
			pane = window.Panes[window.ActivePN]
		}
		// Fallback to first non-nil pane if ActivePN pane is nil or out of range.
		if pane == nil {
			for _, p := range window.Panes {
				if p != nil {
					pane = p
					break
				}
			}
		}
	}
	return expandFormat(format, pane)
}

func formatPaneLine(pane *TmuxPane, customFormat string) string {
	format := strings.TrimSpace(customFormat)
	if format == "" {
		format = defaultPaneListFormat
	}
	return expandFormat(format, pane)
}

func firstPaneInSession(session *TmuxSession) *TmuxPane {
	window := activeWindowInSession(session)
	if window == nil || len(window.Panes) == 0 {
		return nil
	}
	pane, err := activePaneInWindow(window)
	if err != nil {
		return nil
	}
	return pane
}

// expandFormat expands tmux-like #{var} placeholders.
//
// I-11 naming note: This function and its callees (lookupFormatVariable,
// formatSessionLine, etc.) do NOT carry a "Locked" suffix because they are
// pure functions operating on already-cloned snapshots or value parameters.
// They do not access SessionManager fields and therefore have no lock
// requirement. See session_manager_windows.go for the "Locked"/"RLocked"
// suffix convention used by SessionManager methods that operate under lock.
//
// NOTE (S-46): Nested #{...} placeholders are supported via manual brace-matching
// parser (expandFormatNested). This enables comparison operators like
// #{==:#{session_name},value} where inner #{var} references are expanded
// before the comparison is evaluated.
func expandFormat(format string, pane *TmuxPane) string {
	return expandFormatNested(format, pane)
}

// expandFormatNested expands #{...} placeholders with support for nested braces.
// Unlike regex-based expansion, this parser correctly handles nested #{...} inside
// comparison operators like #{==:#{session_name},demo}.
func expandFormatNested(format string, pane *TmuxPane) string {
	var out strings.Builder
	out.Grow(len(format))
	i := 0
	for i < len(format) {
		// Look for "#{" start marker.
		if i+1 < len(format) && format[i] == '#' && format[i+1] == '{' {
			// Find the matching closing brace, respecting nesting.
			inner, end := extractNestedBraces(format, i+2)
			if end < 0 {
				// No matching close brace; emit the rest as-is.
				slog.Debug("[DEBUG-FORMAT] expandFormatNested: unclosed brace in format",
					"snippet", format[i:])
				out.WriteString(format[i:])
				break
			}
			out.WriteString(resolveFormatExpr(inner, pane))
			i = end + 1 // skip past the closing '}'
		} else {
			out.WriteByte(format[i])
			i++
		}
	}
	return out.String()
}

// extractNestedBraces extracts the content between braces starting at position start,
// respecting nested #{...} pairs. Returns (inner content, index of closing brace).
// Returns ("", -1) if no matching closing brace is found.
func extractNestedBraces(s string, start int) (string, int) {
	depth := 1
	i := start
	for i < len(s) {
		if i+1 < len(s) && s[i] == '#' && s[i+1] == '{' {
			depth++
			i += 2
			continue
		}
		if s[i] == '}' {
			depth--
			if depth == 0 {
				return s[start:i], i
			}
		}
		i++
	}
	return "", -1
}

// resolveFormatExpr resolves a single format expression (the content inside #{...}).
// Handles comparison operators (==, !=) and plain variable names.
func resolveFormatExpr(expr string, pane *TmuxPane) string {
	// Handle comparison operators: ==:lhs,rhs and !=:lhs,rhs
	if result, ok := evaluateComparisonExpr(expr, pane); ok {
		return result
	}
	return lookupFormatVariable(expr, pane)
}

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

func lookupFormatVariable(name string, pane *TmuxPane) string {
	if pane == nil {
		switch name {
		case "session_name", "window_name", "pane_id", "pane_tty":
			return ""
		case "session_windows", "window_index", "window_panes", "window_active", "pane_index", "pane_width", "pane_height", "pane_active", "session_created":
			return "0"
		case "pane_active_suffix":
			return ""
		default:
			return ""
		}
	}

	window := pane.Window
	var session *TmuxSession
	if window != nil {
		session = window.Session
	}

	switch name {
	case "pane_id":
		return pane.IDString()
	case "pane_index":
		return strconv.Itoa(pane.Index)
	case "pane_width":
		return strconv.Itoa(pane.Width)
	case "pane_height":
		return strconv.Itoa(pane.Height)
	case "pane_active":
		if pane.Active {
			return "1"
		}
		return "0"
	case "pane_tty":
		return pane.ttyPath()
	case "pane_active_suffix":
		if pane.Active {
			return " (active)"
		}
		return ""
	case "pane_title":
		return pane.Title
	case "window_index":
		// COMPATIBILITY DEVIATION (I-13): tmux's #{window_index} returns a
		// 0-based sequential position that shifts when windows are reordered
		// or removed. This implementation intentionally deviates by returning
		// the stable window.ID, which never changes for the lifetime of a
		// window. This means:
		//   - tmux: window_index may be 0,1,2 and can reset after removal
		//   - myT-x: window_index equals window.ID (monotonically increasing, never reused)
		// Callers that need the positional index should compute it from the
		// session's Windows slice. Callers targeting a specific window should
		// use #{window_id} (prefixed with '@') for unambiguous targeting.
		if window == nil {
			return "0"
		}
		return strconv.Itoa(window.ID)
	case "window_name":
		if window == nil {
			return ""
		}
		return window.Name
	case "window_panes":
		if window == nil {
			return "0"
		}
		return strconv.Itoa(len(window.Panes))
	case "window_active":
		if window == nil || session == nil {
			return "0"
		}
		activeWindow := activeWindowInSession(session)
		if activeWindow != nil && activeWindow.ID == window.ID {
			return "1"
		}
		return "0"
	case "session_name":
		if session == nil {
			return ""
		}
		return session.Name
	case "session_windows":
		if session == nil {
			return "0"
		}
		return strconv.Itoa(len(session.Windows))
	case "session_created":
		if session == nil {
			return "0"
		}
		return strconv.FormatInt(session.CreatedAt.Unix(), 10)
	case "session_created_human":
		if session == nil {
			// Use the same human-readable layout as the non-nil path to keep
			// format output consistent regardless of whether session is set.
			return time.Unix(0, 0).Format("Mon Jan _2 15:04:05 2006")
		}
		return session.CreatedAt.Format("Mon Jan _2 15:04:05 2006")
	default:
		return ""
	}
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

// defaultBufferListFormat is the default format for list-buffers output.
const defaultBufferListFormat = "#{buffer_name}: #{buffer_size} bytes: \"#{buffer_sample}\""

// formatBufferLine formats a paste buffer for list-buffers output.
// Uses expandBufferFormat which supports nested #{...} and comparison operators.
func formatBufferLine(buf *PasteBuffer, customFormat string) string {
	format := strings.TrimSpace(customFormat)
	if format == "" {
		format = defaultBufferListFormat
	}
	return expandBufferFormat(format, buf)
}

// expandBufferFormat expands #{...} placeholders for buffer variables.
// Uses expandFormatNested-style manual brace-matching to correctly handle
// nested #{...} inside comparison operators like #{==:#{buffer_name},foo}.
func expandBufferFormat(format string, buf *PasteBuffer) string {
	var out strings.Builder
	out.Grow(len(format))
	i := 0
	for i < len(format) {
		if i+1 < len(format) && format[i] == '#' && format[i+1] == '{' {
			inner, end := extractNestedBraces(format, i+2)
			if end < 0 {
				slog.Debug("[DEBUG-FORMAT] expandBufferFormat: unclosed brace in format",
					"snippet", format[i:])
				out.WriteString(format[i:])
				break
			}
			out.WriteString(resolveBufferFormatExpr(inner, buf))
			i = end + 1
		} else {
			out.WriteByte(format[i])
			i++
		}
	}
	return out.String()
}

// resolveBufferFormatExpr resolves a single format expression for buffer context.
// Handles comparison operators (==, !=) and plain buffer variable names.
func resolveBufferFormatExpr(expr string, buf *PasteBuffer) string {
	// Handle comparison operators: ==:lhs,rhs and !=:lhs,rhs
	var op string
	var rest string
	if strings.HasPrefix(expr, "==:") {
		op = "=="
		rest = expr[3:]
	} else if strings.HasPrefix(expr, "!=:") {
		op = "!="
		rest = expr[3:]
	} else {
		return lookupBufferVariable(expr, buf)
	}

	commaIdx := findTopLevelComma(rest)
	if commaIdx < 0 {
		slog.Debug("[DEBUG-FORMAT] malformed buffer comparison expr: missing comma", "op", op, "expr", expr)
		return ""
	}
	lhs := expandBufferFormat(rest[:commaIdx], buf)
	rhs := expandBufferFormat(rest[commaIdx+1:], buf)

	switch op {
	case "==":
		if lhs == rhs {
			return "1"
		}
		return "0"
	case "!=":
		if lhs != rhs {
			return "1"
		}
		return "0"
	default:
		return ""
	}
}

func lookupBufferVariable(name string, buf *PasteBuffer) string {
	if buf == nil {
		switch name {
		case "buffer_name":
			return ""
		case "buffer_size":
			return "0"
		case "buffer_sample":
			return ""
		default:
			return ""
		}
	}
	switch name {
	case "buffer_name":
		return buf.Name
	case "buffer_size":
		return strconv.Itoa(len(buf.Data))
	case "buffer_sample":
		sample := string(buf.Data)
		if len(sample) > 50 {
			sample = sample[:50]
		}
		// Replace newlines with spaces for single-line display.
		sample = strings.ReplaceAll(sample, "\n", " ")
		sample = strings.ReplaceAll(sample, "\r", "")
		return sample
	default:
		return ""
	}
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return fmt.Sprintf("%s\n", strings.Join(lines, "\n"))
}
