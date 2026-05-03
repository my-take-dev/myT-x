package main

import (
	"log/slog"
	"strings"
)

// SendDiffReview sends frontend-formatted diff review text to the specified
// pane using the same literal send-keys flow as the chat input bar.
//
// Diff Review is a precomposed multiline Markdown payload. It applies a
// Claude Code pane display workaround before sending because observed literal
// multiline delivery displays lines in reverse order in the target TUI. Chat,
// Orchestrator, Task Scheduler, and Single Task Runner keep their existing
// transport behavior because their input contracts differ and have not shown
// the same Diff Review Markdown ordering failure.
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
	sendText := reverseLinesForLiteralSendKeysWorkaround(text)
	slog.Debug("[DIFF-REVIEW] SendDiffReview applying literal send-keys workaround",
		"paneID", paneID,
		"logicalLen", len(text),
		"sendLen", len(sendText),
		"lineCount", strings.Count(sendText, "\n")+1,
		"linesReversed", sendText != text)
	if err := a.sendKeys.sendKeysLiteralWithEnter(router, paneID, sendText); err != nil {
		slog.Debug("[DIFF-REVIEW] SendDiffReview failed",
			"paneID", paneID,
			"logicalLen", len(text),
			"sendLen", len(sendText),
			"lineCount", strings.Count(sendText, "\n")+1,
			"linesReversed", sendText != text,
			"err", err)
		return err
	}
	sessionName := a.resolveSessionNameForPane(sessions, paneID)
	a.recordInput(paneID, text, "diff-review", sessionName)
	return nil
}

// reverseLinesForLiteralSendKeysWorkaround prepares Diff Review Markdown for
// the current Claude Code literal send-keys pane behavior, where multiline
// text has been observed to display bottom-to-top. This intentionally produces
// transport text that is not valid Markdown by itself; the receiving TUI's
// observed display order restores the logical order. Remove this workaround
// only after manual verification shows literal multiline Diff Review Markdown,
// including blank lines and fenced code blocks, displays in logical order.
func reverseLinesForLiteralSendKeysWorkaround(text string) string {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) <= 1 {
		return normalized
	}
	for left, right := 0, len(lines)-1; left < right; left, right = left+1, right-1 {
		lines[left], lines[right] = lines[right], lines[left]
	}
	return strings.Join(lines, "\n")
}
