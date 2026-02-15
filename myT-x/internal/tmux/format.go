package tmux

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var formatVarPattern = regexp.MustCompile(`#\{([^}]+)\}`)

const (
	defaultSessionListFormat = "#{session_name}: #{session_windows} windows (created #{session_created_human})"
	defaultPaneListFormat    = "#{pane_index}: [#{pane_width}x#{pane_height}] [history 0/2000, 0 bytes] #{pane_id}#{pane_active_suffix}"
)

func formatSessionLine(session *TmuxSession, customFormat string) string {
	format := strings.TrimSpace(customFormat)
	if format == "" {
		format = defaultSessionListFormat
	}

	createdHuman := session.CreatedAt.Format("Mon Jan _2 15:04:05 2006")
	out := strings.ReplaceAll(format, "#{session_created_human}", createdHuman)
	return expandFormat(out, firstPaneInSession(session))
}

func formatPaneLine(pane *TmuxPane, customFormat string) string {
	format := strings.TrimSpace(customFormat)
	if format == "" {
		format = defaultPaneListFormat
	}
	return expandFormat(format, pane)
}

func firstPaneInSession(session *TmuxSession) *TmuxPane {
	if session == nil || len(session.Windows) == 0 || len(session.Windows[0].Panes) == 0 {
		return nil
	}
	return session.Windows[0].Panes[0]
}

// expandFormat expands tmux-like #{var} placeholders.
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
		case "session_windows", "window_index", "window_panes", "pane_index", "pane_width", "pane_height", "pane_active", "session_created":
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
			return time.Unix(0, 0).Format(time.RFC3339)
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
