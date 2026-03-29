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

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return fmt.Sprintf("%s\n", strings.Join(lines, "\n"))
}
