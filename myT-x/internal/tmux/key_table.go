package tmux

import (
	"fmt"
	"log/slog"
	"strings"
)

// sendKeysTable maps named key literals (all lowercase) to their byte sequences.
// Keys are matched via normalizeSendKeyToken, which lowercases and trims input.
//
// The c-c, c-d, c-z, c-[ entries intentionally duplicate coverage provided by
// parseControlKey. They exist in the table for two reasons:
//  1. Table lookup is O(1) and avoids the parseControlKey function call overhead.
//  2. They serve as documentation of the most commonly used control sequences.
//
// parseControlKey acts as a fallback for c-{letter} patterns not explicitly
// listed here (c-a through c-z, plus special chars @, \, ], ^, _).
// It accepts both uppercase and lowercase letter suffixes (C-A and c-a both
// produce 0x01).
var sendKeysTable = map[string][]byte{
	"enter":   {'\r'},
	"kpenter": {'\r'},
	"c-c":     {0x03},
	"c-d":     {0x04},
	"c-z":     {0x1a},
	"c-[":     {0x1b},
	"escape":  {0x1b},
	"space":   {' '},
	"tab":     {'\t'},
	"bspace":  {0x7f},
}

// copyModeCommandTable maps copy-mode command names (all lowercase) to byte sequences.
// Used by send-keys -X to translate copy-mode commands to terminal input.
// Unknown commands are silently ignored (shim spec: never block on transform failure).
var copyModeCommandTable = map[string][]byte{
	"cancel":         {0x1b},                         // Escape — exits copy mode
	"page-up":        {0x1b, '[', '5', '~'},          // Page Up
	"halfpage-up":    {0x1b, '[', '5', '~'},          // Half page up (mapped to Page Up)
	"page-down":      {0x1b, '[', '6', '~'},          // Page Down
	"halfpage-down":  {0x1b, '[', '6', '~'},          // Half page down (mapped to Page Down)
	"cursor-up":      {0x1b, '[', 'A'},               // Up arrow
	"cursor-down":    {0x1b, '[', 'B'},               // Down arrow
	"cursor-right":   {0x1b, '[', 'C'},               // Right arrow
	"cursor-left":    {0x1b, '[', 'D'},               // Left arrow
	"start-of-line":  {0x1b, '[', 'H'},               // Home
	"end-of-line":    {0x1b, '[', 'F'},               // End
	"history-top":    {0x1b, '[', '1', ';', '5', 'H'}, // Ctrl+Home
	"history-bottom": {0x1b, '[', '1', ';', '5', 'F'}, // Ctrl+End
}

// TranslateCopyModeCommand resolves a copy-mode command name to bytes.
// Returns (bytes, true) if the command is known, (nil, false) otherwise.
func TranslateCopyModeCommand(command string) ([]byte, bool) {
	normalized := normalizeSendKeyToken(command)
	value, ok := copyModeCommandTable[normalized]
	return value, ok
}

// TranslateSendKeys translates tmux send-keys arguments to bytes.
// Each argument is resolved in order: sendKeysTable lookup, then
// parseControlKey fallback, then raw byte passthrough.
func TranslateSendKeys(args []string) []byte {
	if len(args) == 0 {
		return nil
	}

	out := make([]byte, 0, 64)
	for _, arg := range args {
		normalized := normalizeSendKeyToken(arg)
		if value, ok := sendKeysTable[normalized]; ok {
			out = append(out, value...)
			continue
		}
		if b, ok := parseControlKey(normalized); ok {
			out = append(out, b)
			continue
		}
		out = append(out, arg...)
	}
	slog.Debug("[DEBUG-KEYTABLE] TranslateSendKeys result",
		"inputArgs", fmt.Sprintf("%v", args),
		"outputHex", fmt.Sprintf("%x", out),
		"outputLen", len(out),
	)
	return out
}

// normalizeSendKeyToken lowercases and trims whitespace from a send-keys token.
// TrimSpace guards against trailing whitespace from CLI argument tokenization.
func normalizeSendKeyToken(arg string) string {
	return strings.ToLower(strings.TrimSpace(arg))
}

// parseControlKey parses "C-{letter}" (or "c-{letter}") notation into a control byte.
// Accepts both uppercase and lowercase letter suffixes: C-A through C-Z and c-a
// through c-z both map to 0x01 through 0x1a.
// Returns (byte, true) on success.
func parseControlKey(arg string) (byte, bool) {
	if len(arg) != 3 || (arg[0] != 'C' && arg[0] != 'c') || arg[1] != '-' {
		return 0, false
	}
	ch := arg[2]
	switch ch {
	case '@':
		return 0x00, true
	case '\\':
		return 0x1c, true
	case ']':
		return 0x1d, true
	case '^':
		return 0x1e, true
	case '_':
		return 0x1f, true
	}
	if ch >= 'a' && ch <= 'z' {
		return ch - 'a' + 1, true
	}
	if ch >= 'A' && ch <= 'Z' {
		return ch - 'A' + 1, true
	}
	return 0, false
}
