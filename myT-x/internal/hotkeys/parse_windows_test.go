//go:build windows

package hotkeys

import (
	"strings"
	"testing"
	"unsafe"
)

func TestParseBindingSuccess(t *testing.T) {
	tests := []struct {
		name     string
		spec     string
		wantNorm string
		wantMods Modifier
		wantKey  VKey
	}{
		// Function key with two modifiers
		{
			name:     "Ctrl+Shift+F12",
			spec:     "Ctrl+Shift+F12",
			wantNorm: "Ctrl+Shift+F12",
			wantMods: modControl | modShift,
			wantKey:  vkF12,
		},
		// Backtick key
		{
			name:     "Ctrl+backtick",
			spec:     "Ctrl+`",
			wantNorm: "Ctrl+`",
			wantMods: modControl,
			wantKey:  vkOem3,
		},
		// Letter key
		{
			name:     "Ctrl+A",
			spec:     "Ctrl+A",
			wantNorm: "Ctrl+A",
			wantMods: modControl,
			wantKey:  VKey('A'),
		},
		// Digit key
		{
			name:     "Alt+3",
			spec:     "Alt+3",
			wantNorm: "Alt+3",
			wantMods: modAlt,
			wantKey:  VKey('3'),
		},
		// Named key: space
		{
			name:     "Ctrl+Space",
			spec:     "Ctrl+Space",
			wantNorm: "Ctrl+SPACE",
			wantMods: modControl,
			wantKey:  vkSpace,
		},
		// Named key: tab
		{
			name:     "Alt+Tab",
			spec:     "Alt+Tab",
			wantNorm: "Alt+TAB",
			wantMods: modAlt,
			wantKey:  vkTab,
		},
		// Named key: enter
		{
			name:     "Ctrl+Enter",
			spec:     "Ctrl+Enter",
			wantNorm: "Ctrl+ENTER",
			wantMods: modControl,
			wantKey:  vkReturn,
		},
		// Named key: delete
		{
			name:     "Ctrl+Delete",
			spec:     "Ctrl+Delete",
			wantNorm: "Ctrl+DELETE",
			wantMods: modControl,
			wantKey:  vkDelete,
		},
		// Arrow key
		{
			name:     "Ctrl+Left",
			spec:     "Ctrl+Left",
			wantNorm: "Ctrl+LEFT",
			wantMods: modControl,
			wantKey:  vkLeft,
		},
		// Hex virtual-key code
		{
			name:     "Ctrl+0x41 (hex A)",
			spec:     "Ctrl+0x41",
			wantNorm: "Ctrl+0X41",
			wantMods: modControl,
			wantKey:  VKey(0x41),
		},
		// BACKQUOTE alias
		{
			name:     "Ctrl+Backquote",
			spec:     "Ctrl+Backquote",
			wantNorm: "Ctrl+`",
			wantMods: modControl,
			wantKey:  vkOem3,
		},
		// GRAVE alias
		{
			name:     "Ctrl+Grave",
			spec:     "Ctrl+Grave",
			wantNorm: "Ctrl+`",
			wantMods: modControl,
			wantKey:  vkOem3,
		},
		// Modifier aliases: Control == Ctrl
		{
			name:     "Control+A alias",
			spec:     "Control+A",
			wantNorm: "Ctrl+A",
			wantMods: modControl,
			wantKey:  VKey('A'),
		},
		// Modifier aliases: Super == Win
		{
			name:     "Super+A alias",
			spec:     "Super+A",
			wantNorm: "Win+A",
			wantMods: modWin,
			wantKey:  VKey('A'),
		},
		// All four modifiers
		{
			name:     "all modifiers",
			spec:     "Ctrl+Alt+Shift+Win+A",
			wantNorm: "Ctrl+Alt+Shift+Win+A",
			wantMods: modControl | modAlt | modShift | modWin,
			wantKey:  VKey('A'),
		},
		// Duplicate modifiers deduplicated
		{
			name:     "dedup Ctrl+Ctrl+A",
			spec:     "Ctrl+Ctrl+A",
			wantNorm: "Ctrl+A",
			wantMods: modControl,
			wantKey:  VKey('A'),
		},
		// Case insensitivity
		{
			name:     "lowercase ctrl+shift+f12",
			spec:     "ctrl+shift+f12",
			wantNorm: "Ctrl+Shift+F12",
			wantMods: modControl | modShift,
			wantKey:  vkF12,
		},
		// Whitespace padding
		{
			name:     "whitespace padded",
			spec:     "  Ctrl + A  ",
			wantNorm: "Ctrl+A",
			wantMods: modControl,
			wantKey:  VKey('A'),
		},
		// F1 function key
		{
			name:     "Alt+F1",
			spec:     "Alt+F1",
			wantNorm: "Alt+F1",
			wantMods: modAlt,
			wantKey:  vkF1,
		},
		// ESC alias
		{
			name:     "Ctrl+Esc",
			spec:     "Ctrl+Esc",
			wantNorm: "Ctrl+ESC",
			wantMods: modControl,
			wantKey:  vkEscape,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binding, err := ParseBinding(tt.spec)
			if err != nil {
				t.Fatalf("ParseBinding(%q) returned unexpected error: %v", tt.spec, err)
			}
			if binding.Normalized() != tt.wantNorm {
				t.Errorf("Normalized() = %q, want %q", binding.Normalized(), tt.wantNorm)
			}
			if binding.Modifiers() != tt.wantMods {
				t.Errorf("Modifiers() = 0x%X, want 0x%X", binding.Modifiers(), tt.wantMods)
			}
			if binding.Key() != tt.wantKey {
				t.Errorf("Key() = 0x%X, want 0x%X", binding.Key(), tt.wantKey)
			}
		})
	}
}

func TestParseBindingErrors(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		wantSub string // expected substring in error message
	}{
		{
			name:    "empty spec",
			spec:    "",
			wantSub: "empty",
		},
		{
			name:    "whitespace-only spec",
			spec:    "   ",
			wantSub: "empty",
		},
		{
			name:    "key only, no modifier",
			spec:    "Ctrl",
			wantSub: "modifiers and key",
		},
		{
			name:    "unknown modifier",
			spec:    "Meta+A",
			wantSub: "unknown modifier",
		},
		{
			name:    "missing key token",
			spec:    "Ctrl+",
			wantSub: "missing hotkey key token",
		},
		{
			name:    "unknown key name",
			spec:    "Ctrl+PageUp",
			wantSub: "unknown key",
		},
		{
			name:    "invalid hex key",
			spec:    "Ctrl+0xZZZZ",
			wantSub: "invalid hex key",
		},
		{
			name:    "hex key 0x0000",
			spec:    "Ctrl+0x0000",
			wantSub: "not a valid virtual key",
		},
		{
			name:    "all duplicate modifiers => zero (leading +)",
			spec:    "+A",
			wantSub: "unknown modifier",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseBinding(tt.spec)
			if err == nil {
				t.Fatalf("ParseBinding(%q) expected error, got nil", tt.spec)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.wantSub)
			}
		})
	}
}

// TestWinMsgSize verifies that the winMsg struct matches the Win32 MSG layout.
func TestWinMsgSize(t *testing.T) {
	// On amd64 (64-bit): 48 bytes. On 386 (32-bit): 28 bytes.
	ptrSize := unsafe.Sizeof(uintptr(0))
	var expectedSize uintptr
	switch ptrSize {
	case 8: // 64-bit
		expectedSize = 48
	case 4: // 32-bit
		expectedSize = 28
	default:
		t.Skipf("unknown pointer size %d", ptrSize)
	}
	if got := unsafe.Sizeof(winMsg{}); got != expectedSize {
		t.Fatalf("unsafe.Sizeof(winMsg{}) = %d, want %d (pointer size=%d)", got, expectedSize, ptrSize)
	}
}
