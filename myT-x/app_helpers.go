package main

import (
	"fmt"
	"strings"

	"myT-x/internal/tmux"
)

// isPaneAlive checks whether a pane with the given ID exists in any session.
func isPaneAlive(sessions *tmux.SessionManager, paneID string) bool {
	_, err := paneContextSnapshot(sessions, paneID)
	return err == nil
}

func paneContextSnapshot(sessions *tmux.SessionManager, paneID string) (tmux.PaneContextSnapshot, error) {
	return sessions.GetPaneContextSnapshot(tmux.ParseCallerPane(strings.TrimSpace(paneID)))
}

func requirePaneInSession(sessions *tmux.SessionManager, sessionName, paneID string) error {
	sessionName = strings.TrimSpace(sessionName)
	paneID = strings.TrimSpace(paneID)
	if sessionName == "" {
		return fmt.Errorf("session name is required")
	}
	if paneID == "" {
		return fmt.Errorf("pane id is required")
	}

	paneCtx, err := paneContextSnapshot(sessions, paneID)
	if err != nil {
		return fmt.Errorf("pane %s does not exist: %w", paneID, err)
	}
	if strings.TrimSpace(paneCtx.SessionName) != sessionName {
		return fmt.Errorf("pane %s belongs to session %s, not %s", paneID, paneCtx.SessionName, sessionName)
	}
	return nil
}
