package main

import (
	"myT-x/internal/tmux"
)

// isPaneAlive checks whether a pane with the given ID exists in any session.
func isPaneAlive(sessions *tmux.SessionManager, paneID string) bool {
	for _, sess := range sessions.Snapshot() {
		for _, win := range sess.Windows {
			for _, pane := range win.Panes {
				if pane.ID == paneID {
					return true
				}
			}
		}
	}
	return false
}
