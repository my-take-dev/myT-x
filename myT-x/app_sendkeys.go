package main

import (
	"fmt"
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

// sendKeysLiteralSleepFn is a test seam for time.Sleep in send-keys helpers.
var sendKeysLiteralSleepFn = time.Sleep

// sendKeysLiteralWithEnter sends text to a pane using the 3-step pattern:
//  1. Send literal text in chunks (500 runes each, 300ms delay between chunks).
//  2. Wait 300ms.
//  3. Send Enter (C-m) separately.
//
// This ensures interactive prompts receive the full message before Enter,
// preventing paste-style bulk submission issues.
func sendKeysLiteralWithEnter(router *tmux.CommandRouter, paneID string, text string) error {
	// Step 1: Send literal text (no key interpretation) in chunks.
	runes := []rune(text)
	for i := 0; i < len(runes); i += sendKeysLiteralChunkSize {
		end := min(i+sendKeysLiteralChunkSize, len(runes))
		chunk := string(runes[i:end])
		resp := executeRouterRequestFn(router, ipc.TmuxRequest{
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
			sendKeysLiteralSleepFn(sendKeysLiteralDelay)
		}
	}

	// Step 2: Wait before sending Enter.
	sendKeysLiteralSleepFn(sendKeysLiteralDelay)

	// Step 3: Send Enter (C-m) separately.
	resp := executeRouterRequestFn(router, ipc.TmuxRequest{
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
// Flow: ESC[200~ → text chunks → ESC[201~ → C-m
func sendKeysLiteralPasteWithEnter(router *tmux.CommandRouter, paneID string, text string) error {
	// Step 1: Send paste mode start: ESC[200~
	resp := executeRouterRequestFn(router, ipc.TmuxRequest{
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
		resp := executeRouterRequestFn(router, ipc.TmuxRequest{
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
			sendKeysLiteralSleepFn(sendKeysLiteralDelay)
		}
	}

	// Step 3: Send paste mode end: ESC[201~
	sendKeysLiteralSleepFn(sendKeysLiteralDelay)
	resp = executeRouterRequestFn(router, ipc.TmuxRequest{
		Command: "send-keys",
		Flags: map[string]any{
			"-t": paneID,
			"-l": true,
		},
		Args: []string{bracketedPasteEnd},
	})
	if resp.ExitCode != 0 {
		return fmt.Errorf("send-keys paste-end failed: %s", strings.TrimSpace(resp.Stderr))
	}

	// Step 4: Send Enter (C-m) separately.
	sendKeysLiteralSleepFn(sendKeysLiteralDelay)
	resp = executeRouterRequestFn(router, ipc.TmuxRequest{
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
