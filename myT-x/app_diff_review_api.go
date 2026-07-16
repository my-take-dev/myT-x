package main

import (
	"log/slog"
	"strings"
)

// SendDiffReview sends frontend-formatted diff review text to the specified
// pane using the same literal send-keys flow as the chat input bar.
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
	if err := a.sendKeys.sendKeysLiteralWithEnter(router, paneID, text); err != nil {
		slog.Debug("[DIFF-REVIEW] SendDiffReview failed",
			"paneID", paneID,
			"textLen", len(text),
			"lineCount", strings.Count(text, "\n")+1,
			"err", err)
		return err
	}
	sessionName := a.resolveSessionNameForPane(sessions, paneID)
	a.recordInput(paneID, text, "diff-review", sessionName)
	return nil
}
