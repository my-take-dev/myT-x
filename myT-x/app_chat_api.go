package main

import (
	"log/slog"
	"strings"
)

// SendChatMessage sends a chat message to the specified pane using the 3-step
// send-keys pattern: literal text in chunks → delay → Enter (C-m).
// This is the Wails binding called from the frontend chat input bar.
func (a *App) SendChatMessage(paneID string, text string) error {
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
		slog.Debug("[CHAT] SendChatMessage failed", "paneID", paneID, "err", err)
		return err
	}
	sessionName := a.resolveSessionNameForPane(sessions, paneID)
	a.recordInput(paneID, text, "chat", sessionName)
	return nil
}
