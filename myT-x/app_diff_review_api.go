package main

import (
	"log/slog"
	"strings"
)

// SendDiffReview sends review comments to the specified pane using bracketed
// paste mode. The frontend formats comments as Markdown before calling this
// method. Bracketed paste ensures multi-line content is received as a single
// paste event by interactive AI tools (e.g., Claude Code).
func (a *App) SendDiffReview(paneID string, text string) error {
	sessions, err := a.requireSessionsWithPaneID(&paneID)
	if err != nil {
		return err
	}
	text = strings.TrimRight(text, "\r\n")
	if text == "" {
		return nil
	}
	router, err := a.requireRouter()
	if err != nil {
		return err
	}
	if err := a.sendKeys.sendKeysLiteralPasteWithEnter(router, paneID, text); err != nil {
		slog.Debug("[DIFF-REVIEW] SendDiffReview failed", "paneID", paneID, "err", err)
		return err
	}
	sessionName := a.resolveSessionNameForPane(sessions, paneID)
	a.recordInput(paneID, text, "diff-review", sessionName)
	return nil
}
