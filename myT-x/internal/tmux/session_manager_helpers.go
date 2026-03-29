package tmux

import (
	"fmt"
	"log/slog"
	"maps"
	"regexp"
	"strconv"
	"strings"
)

// copyBoolPtr returns a shallow copy of a *bool pointer.
// Returns nil when src is nil.
func copyBoolPtr(src *bool) *bool {
	if src == nil {
		return nil
	}
	v := *src
	return &v
}

func copyEnvMap(input map[string]string) map[string]string {
	// Preserve caller safety by always returning a mutable map:
	// nil/empty input -> empty non-nil map.
	if len(input) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(input))
	maps.Copy(out, input)
	return out
}

func sessionWorkDir(session *TmuxSession) string {
	if session == nil {
		return ""
	}
	workDir := strings.TrimSpace(session.RootPath)
	if wt := session.Worktree; wt != nil && strings.TrimSpace(wt.Path) != "" {
		workDir = strings.TrimSpace(wt.Path)
	}
	return workDir
}

// ParseCallerPane parses a TMUX_PANE-like id string.
func ParseCallerPane(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return -1
	}
	id, err := parsePaneID(value)
	if err != nil {
		return -1
	}
	return id
}

func parsePaneID(value string) (int, error) {
	if !strings.HasPrefix(value, "%") {
		return -1, fmt.Errorf("invalid pane id: %s", value)
	}
	id, err := strconv.Atoi(strings.TrimPrefix(value, "%"))
	if err != nil || id < 0 {
		return -1, fmt.Errorf("invalid pane id: %s", value)
	}
	return id, nil
}

// Compiled at package level to avoid re-compilation per call.
var (
	sessionNameSanitizer = regexp.MustCompile(`[.:]+`)
	consecutiveHyphen    = regexp.MustCompile(`-{2,}`)
)

// SanitizeSessionName replaces characters that are invalid in tmux session
// names with hyphens. tmux rejects names containing '.' and ':'.
// fallback is returned when the sanitized name is empty.
func SanitizeSessionName(name, fallback string) string {
	sanitized := sessionNameSanitizer.ReplaceAllString(name, "-")
	sanitized = consecutiveHyphen.ReplaceAllString(sanitized, "-")
	sanitized = strings.Trim(sanitized, "-")
	if sanitized == "" {
		return fallback
	}
	return sanitized
}

// ResolveActiveWindow returns the window matching activeWindowID, or the first
// window as a fallback. Returns nil if windows is empty.
func ResolveActiveWindow(windows []WindowSnapshot, activeWindowID int) *WindowSnapshot {
	if len(windows) == 0 {
		return nil
	}
	for i := range windows {
		if windows[i].ID == activeWindowID {
			return &windows[i]
		}
	}
	slog.Debug("[DEBUG-TMUX] activeWindowID not found in windows, falling back to first window",
		"activeWindowID", activeWindowID,
		"windowCount", len(windows),
		"firstWindowID", windows[0].ID)
	return &windows[0]
}
