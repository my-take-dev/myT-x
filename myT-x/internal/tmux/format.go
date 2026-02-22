package tmux

import (
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var formatVarPattern = regexp.MustCompile(`#\{([^}]+)\}`)

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
// NOTE (S-46): Nested #{...} placeholders (e.g. "#{window_#{idx}}") are NOT
// supported. The regex-based expansion performs a single pass; nested braces
// will produce an empty string for the unrecognised inner variable name.
func expandFormat(format string, pane *TmuxPane) string {
	return formatVarPattern.ReplaceAllStringFunc(format, func(match string) string {
		parts := formatVarPattern.FindStringSubmatch(match)
		if len(parts) != 2 {
			return ""
		}
		return lookupFormatVariable(parts[1], pane)
	})
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

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return fmt.Sprintf("%s\n", strings.Join(lines, "\n"))
}
