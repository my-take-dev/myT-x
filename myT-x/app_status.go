package main

import (
	"fmt"
	"log/slog"
	"time"

	"myT-x/internal/tmux"
)

// resolvePaneTitle determines the display title for the status line.
//
// Fallback priority (first non-empty value wins):
//  1. Active pane matched by activePN index -> Title
//  2. Active pane matched by Active flag    -> Title  (fallback when activePN is stale)
//  3. Any pane with a non-empty Title       -> Title  (fallback when active pane has no title)
//  4. Active pane ID                        -> ID     (fallback when no pane has a title)
//  5. First pane ID                         -> ID     (last resort)
//  6. ""                                    -> empty  (no panes at all)
func resolvePaneTitle(panes []tmux.PaneSnapshot, activePN int) string {
	// Priority 1: find the active pane by activePN (0-based Panes slice index).
	// pane.Index equals the pane's position in the slice, so this comparison is
	// equivalent to a direct slice lookup but avoids panic on out-of-range values.
	var activePaneSnapshot *tmux.PaneSnapshot
	for i := range panes {
		pane := &panes[i]
		if pane.Index != activePN {
			continue
		}
		activePaneSnapshot = pane
		break
	}
	// Priority 2: activePN did not match; fall back to the pane with Active flag.
	if activePaneSnapshot == nil {
		for i := range panes {
			pane := &panes[i]
			if !pane.Active {
				continue
			}
			activePaneSnapshot = pane
			break
		}
	}
	// Priority 1-2 result: use the active pane's title if available.
	if activePaneSnapshot != nil && activePaneSnapshot.Title != "" {
		return activePaneSnapshot.Title
	}
	// Priority 3: any pane with a non-empty title.
	for _, pane := range panes {
		if pane.Title == "" {
			continue
		}
		return pane.Title
	}
	// Priority 4: active pane ID as a last-resort identifier.
	if activePaneSnapshot != nil && activePaneSnapshot.ID != "" {
		return activePaneSnapshot.ID
	}
	// Priority 5: first pane ID.
	if len(panes) > 0 {
		return panes[0].ID
	}
	// Priority 6: no panes exist.
	return ""
}

func resolveActiveWindowSnapshot(windows []tmux.WindowSnapshot, activeWindowID int) *tmux.WindowSnapshot {
	if len(windows) == 0 {
		return nil
	}
	for i := range windows {
		window := &windows[i]
		if window.ID == activeWindowID {
			return window
		}
	}
	// Fallback: activeWindowID did not match any window. This can happen transiently
	// when a window is destroyed between snapshot collection and status rendering.
	// Log the mismatch for diagnostics and fall back to the first window, which is
	// always safe because snapshot windows never contain nil entries.
	slog.Debug("[DEBUG-STATUS] activeWindowID not found in windows, falling back to first window",
		"activeWindowID", activeWindowID,
		"windowCount", len(windows),
		"firstWindowID", windows[0].ID)
	return &windows[0]
}

// BuildStatusLine returns status line data.
func (a *App) BuildStatusLine() string {
	const noSessionLabel = "[セッションなし]"
	sessionsManager, err := a.requireSessions()
	if err != nil {
		return fmt.Sprintf("%s | %s", noSessionLabel, time.Now().Format("15:04"))
	}
	sessions := sessionsManager.Snapshot()
	if len(sessions) == 0 {
		return fmt.Sprintf("%s | %s", noSessionLabel, time.Now().Format("15:04"))
	}

	var current tmux.SessionSnapshot
	found := false
	activeSession := a.getActiveSessionName()
	for _, session := range sessions {
		if session.Name == activeSession {
			current = session
			found = true
			break
		}
	}
	if !found {
		current = sessions[0]
	}

	windowID := 0
	paneTitle := ""
	if activeWindow := resolveActiveWindowSnapshot(current.Windows, current.ActiveWindowID); activeWindow != nil {
		windowID = activeWindow.ID
		paneTitle = resolvePaneTitle(activeWindow.Panes, activeWindow.ActivePN)
	}
	if paneTitle == "" {
		paneTitle = "ペイン"
	}
	// NOTE: time.Now() is called at render time to display the current clock.
	// This is intentionally not cached because BuildStatusLine is called on demand
	// and the displayed time should reflect the moment the status line is rendered.
	return fmt.Sprintf("[%s] <%d> \"%s\" | %s", current.Name, windowID, paneTitle, time.Now().Format("15:04"))
}
