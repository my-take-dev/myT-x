package main

import (
	"fmt"
	"time"

	"myT-x/internal/tmux"
)

func resolvePaneTitle(panes []tmux.PaneSnapshot, activePN int) string {
	var activePaneSnapshot *tmux.PaneSnapshot
	for i := range panes {
		pane := &panes[i]
		// activePN is the 0-based Panes slice index of the active pane (WindowSnapshot.ActivePN).
		// pane.Index equals the pane's position in the slice, so this comparison is equivalent
		// to a direct slice lookup but avoids panic on out-of-range values.
		if pane.Index != activePN {
			continue
		}
		activePaneSnapshot = pane
		break
	}
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
	if activePaneSnapshot != nil && activePaneSnapshot.Title != "" {
		return activePaneSnapshot.Title
	}
	for _, pane := range panes {
		if pane.Title == "" {
			continue
		}
		return pane.Title
	}
	if activePaneSnapshot != nil && activePaneSnapshot.ID != "" {
		return activePaneSnapshot.ID
	}
	if len(panes) > 0 {
		return panes[0].ID
	}
	return ""
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

	windowIdx := 0
	paneTitle := ""
	if len(current.Windows) > 0 {
		windowIdx = current.Windows[0].ID
		paneTitle = resolvePaneTitle(current.Windows[0].Panes, current.Windows[0].ActivePN)
	}
	if paneTitle == "" {
		paneTitle = "ペイン"
	}
	return fmt.Sprintf("[%s] <%d> \"%s\" | %s", current.Name, windowIdx, paneTitle, time.Now().Format("15:04"))
}
