package main

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"myT-x/internal/ipc"
	"myT-x/internal/tmux"
)

const (
	// sendKeysLiteralDelay is the delay between chunks and before Enter.
	sendKeysLiteralDelay = 300 * time.Millisecond
	// sendKeysLiteralChunkSize is the maximum number of runes per send-keys call.
	sendKeysLiteralChunkSize = 500
)

// Bracketed paste mode escape sequences.
// When text is wrapped in these sequences, terminal applications treat
// embedded \n as line breaks within pasted text rather than as Enter/submit.
const (
	bracketedPasteStart = "\x1b[200~"
	bracketedPasteEnd   = "\x1b[201~"
)

// sendKeysIO holds injectable functions for send-keys operations.
// Tests inject custom implementations to avoid real router calls and sleep delays.
type sendKeysIO struct {
	// executeRequest dispatches a tmux command to the router.
	executeRequest func(*tmux.CommandRouter, ipc.TmuxRequest) ipc.TmuxResponse
	// sleep pauses execution for the given duration between send-keys steps.
	sleep func(time.Duration)
}

// defaultSendKeysIO returns send-keys IO with stdlib defaults.
func defaultSendKeysIO() sendKeysIO {
	return sendKeysIO{
		executeRequest: func(router *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
			return router.Execute(req)
		},
		sleep: time.Sleep,
	}
}

// selectPane focuses the target pane before sending keys.
// This matches the MCP executor behavior and ensures ConPTY focus events
// are delivered to the correct pane during multi-pane operations.
func (sk sendKeysIO) selectPane(router *tmux.CommandRouter, paneID string) error {
	resp := sk.executeRequest(router, ipc.TmuxRequest{
		Command: "select-pane",
		Flags: map[string]any{
			"-t": paneID,
		},
	})
	if resp.ExitCode != 0 {
		return fmt.Errorf("select-pane failed: %s", strings.TrimSpace(resp.Stderr))
	}
	return nil
}

// sendKeysLiteralWithEnter sends text to a pane using the 4-step pattern:
//  1. Focus the target pane (select-pane).
//  2. Send literal text in chunks (500 runes each, 300ms delay between chunks).
//  3. Wait 300ms.
//  4. Send Enter (C-m) separately.
//
// This ensures interactive prompts receive the full message before Enter,
// preventing paste-style bulk submission issues.
func (sk sendKeysIO) sendKeysLiteralWithEnter(router *tmux.CommandRouter, paneID string, text string) error {
	// Strip trailing newlines/carriage returns before sending.
	// Enter (C-m) is sent separately in the final step, so trailing \n in
	// the source text would cause a double-submit or break paste boundaries.
	text = strings.TrimRight(text, "\n\r")

	// Step 1: Focus the target pane.
	if err := sk.selectPane(router, paneID); err != nil {
		return err
	}

	// Step 2: Send literal text (no key interpretation) in chunks.
	runes := []rune(text)
	for i := 0; i < len(runes); i += sendKeysLiteralChunkSize {
		end := min(i+sendKeysLiteralChunkSize, len(runes))
		chunk := string(runes[i:end])
		resp := sk.executeRequest(router, ipc.TmuxRequest{
			Command: "send-keys",
			Flags: map[string]any{
				"-t": paneID,
				"-l": true,
			},
			Args: []string{chunk},
		})
		if resp.ExitCode != 0 {
			return fmt.Errorf("send-keys text failed: %s", strings.TrimSpace(resp.Stderr))
		}
		if end < len(runes) {
			sk.sleep(sendKeysLiteralDelay)
		}
	}

	// Step 3: Wait before sending Enter.
	sk.sleep(sendKeysLiteralDelay)

	// Step 4: Send Enter (C-m) separately.
	resp := sk.executeRequest(router, ipc.TmuxRequest{
		Command: "send-keys",
		Flags: map[string]any{
			"-t": paneID,
		},
		Args: []string{"C-m"},
	})
	if resp.ExitCode != 0 {
		return fmt.Errorf("send-keys C-m failed: %s", strings.TrimSpace(resp.Stderr))
	}
	return nil
}

// sendKeysLiteralPasteWithEnter wraps text in bracketed paste mode escape
// sequences before sending. This prevents interactive TUIs (e.g. Claude Code)
// from treating embedded \n as Enter/submit.
//
// Flow: select-pane → ESC[200~ → text chunks → ESC[201~ → C-m
func (sk sendKeysIO) sendKeysLiteralPasteWithEnter(router *tmux.CommandRouter, paneID string, text string) error {
	// Strip trailing newlines/carriage returns before sending.
	// A trailing \n inside bracketed paste (before ESC[201~) can confuse the
	// receiving TUI's input parser, and Enter is sent separately as C-m.
	text = strings.TrimRight(text, "\n\r")

	if err := sk.selectPane(router, paneID); err != nil {
		return err
	}

	// Step 1: Send paste mode start: ESC[200~
	resp := sk.executeRequest(router, ipc.TmuxRequest{
		Command: "send-keys",
		Flags: map[string]any{
			"-t": paneID,
			"-l": true,
		},
		Args: []string{bracketedPasteStart},
	})
	if resp.ExitCode != 0 {
		return fmt.Errorf("send-keys paste-start failed: %s", strings.TrimSpace(resp.Stderr))
	}

	// Step 2: Send literal text in chunks.
	runes := []rune(text)
	for i := 0; i < len(runes); i += sendKeysLiteralChunkSize {
		end := min(i+sendKeysLiteralChunkSize, len(runes))
		chunk := string(runes[i:end])
		chunkResp := sk.executeRequest(router, ipc.TmuxRequest{
			Command: "send-keys",
			Flags: map[string]any{
				"-t": paneID,
				"-l": true,
			},
			Args: []string{chunk},
		})
		if chunkResp.ExitCode != 0 {
			// Text send failed; still send paste-end to leave terminal in a clean state.
			sk.sendPasteEnd(router, paneID)
			return fmt.Errorf("send-keys text failed: %s", strings.TrimSpace(chunkResp.Stderr))
		}
		if end < len(runes) {
			sk.sleep(sendKeysLiteralDelay)
		}
	}

	// Step 3: Send paste-end: ESC[201~
	sk.sleep(sendKeysLiteralDelay)
	sk.sendPasteEnd(router, paneID)

	// Step 4: Send Enter (C-m) separately.
	sk.sleep(sendKeysLiteralDelay)
	enterResp := sk.executeRequest(router, ipc.TmuxRequest{
		Command: "send-keys",
		Flags: map[string]any{
			"-t": paneID,
		},
		Args: []string{"C-m"},
	})
	if enterResp.ExitCode != 0 {
		return fmt.Errorf("send-keys C-m failed: %s", strings.TrimSpace(enterResp.Stderr))
	}
	return nil
}

// schedulerSendMessage sends a message to a pane via the command router,
// using CRLF mode (-N flag) for ConPTY Enter key compatibility.
//
// Flow: select-pane → send-keys -N message Enter
func (sk sendKeysIO) schedulerSendMessage(router *tmux.CommandRouter, paneID, message string) error {
	// Strip trailing newlines/carriage returns; Enter is sent as a separate
	// argument so trailing \n would cause a double-submit.
	message = strings.TrimRight(message, "\n\r")

	if err := sk.selectPane(router, paneID); err != nil {
		return err
	}

	resp := sk.executeRequest(router, ipc.TmuxRequest{
		Command: "send-keys",
		Flags: map[string]any{
			"-t": paneID,
			"-N": true,
		},
		Args: []string{message, "Enter"},
	})
	if resp.ExitCode != 0 {
		return fmt.Errorf("send-keys failed: %s", strings.TrimSpace(resp.Stderr))
	}
	return nil
}

// sendPasteEnd sends the bracketed paste end sequence (best-effort).
// Failure is logged but not propagated to avoid masking the original error.
func (sk sendKeysIO) sendPasteEnd(router *tmux.CommandRouter, paneID string) {
	endResp := sk.executeRequest(router, ipc.TmuxRequest{
		Command: "send-keys",
		Flags: map[string]any{
			"-t": paneID,
			"-l": true,
		},
		Args: []string{bracketedPasteEnd},
	})
	if endResp.ExitCode != 0 {
		slog.Warn("[WARN-SENDKEYS] paste-end send failed",
			"paneID", paneID,
			"stderr", strings.TrimSpace(endResp.Stderr))
	}
}
