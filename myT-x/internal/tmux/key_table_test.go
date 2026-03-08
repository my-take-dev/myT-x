package tmux

import (
	"testing"
)

func TestTranslateSendKeys(t *testing.T) {
	got := TranslateSendKeys([]string{"echo", "Space", "ok", "Enter"})
	want := []byte("echo ok\r")
	if string(got) != string(want) {
		t.Fatalf("TranslateSendKeys() = %q, want %q", string(got), string(want))
	}
}

func TestTranslateSendKeysEnterAliasesCaseInsensitive(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{name: "Enter mixed case", key: "Enter"},
		{name: "enter lowercase", key: "enter"},
		{name: "ENTER uppercase", key: "ENTER"},
		{name: "KPEnter mixed case", key: "KPEnter"},
		{name: "kpenter lowercase", key: "kpenter"},
		{name: "KPENTER uppercase", key: "KPENTER"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TranslateSendKeys([]string{tt.key})
			want := []byte{'\r'}
			if string(got) != string(want) {
				t.Fatalf("TranslateSendKeys([%q]) = %q, want %q", tt.key, got, want)
			}
		})
	}
}

func TestTranslateSendKeysControlKeys(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []byte
	}{
		{
			name: "C-m produces carriage return",
			args: []string{"C-m"},
			want: []byte{0x0D},
		},
		{
			name: "C-a produces SOH",
			args: []string{"C-a"},
			want: []byte{0x01},
		},
		{
			name: "C-l produces form feed",
			args: []string{"C-l"},
			want: []byte{0x0C},
		},
		{
			name: "C-c via table produces ETX",
			args: []string{"C-c"},
			want: []byte{0x03},
		},
		{
			name: "C-d via table produces EOT",
			args: []string{"C-d"},
			want: []byte{0x04},
		},
		{
			name: "C-z via table produces SUB",
			args: []string{"C-z"},
			want: []byte{0x1a},
		},
		{
			name: "C-[ via table produces ESC",
			args: []string{"C-["},
			want: []byte{0x1b},
		},
		{
			name: "mixed text and control key",
			args: []string{"echo", "Space", "hello", "C-m"},
			want: []byte("echo hello\r"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TranslateSendKeys(tt.args)
			if string(got) != string(tt.want) {
				t.Fatalf("TranslateSendKeys(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestParseControlKey(t *testing.T) {
	tests := []struct {
		name     string
		arg      string
		wantByte byte
		wantOK   bool
	}{
		// Valid lowercase
		{name: "C-a lowercase", arg: "C-a", wantByte: 0x01, wantOK: true},
		{name: "C-m lowercase", arg: "C-m", wantByte: 0x0D, wantOK: true},
		{name: "C-z lowercase", arg: "C-z", wantByte: 0x1a, wantOK: true},
		// Valid uppercase
		{name: "C-A uppercase", arg: "C-A", wantByte: 0x01, wantOK: true},
		{name: "C-M uppercase", arg: "C-M", wantByte: 0x0D, wantOK: true},
		{name: "c-m lowercase prefix", arg: "c-m", wantByte: 0x0D, wantOK: true},
		{name: "C-@ special", arg: "C-@", wantByte: 0x00, wantOK: true},
		{name: "C-backslash special", arg: "C-\\", wantByte: 0x1c, wantOK: true},
		{name: "C-right-bracket special", arg: "C-]", wantByte: 0x1d, wantOK: true},
		{name: "C-caret special", arg: "C-^", wantByte: 0x1e, wantOK: true},
		{name: "C-underscore special", arg: "C-_", wantByte: 0x1f, wantOK: true},
		// Invalid inputs
		{name: "digit not letter", arg: "C-1", wantByte: 0, wantOK: false},
		{name: "left bracket must be handled by table", arg: "C-[", wantByte: 0, wantOK: false},
		{name: "missing letter", arg: "C-", wantByte: 0, wantOK: false},
		{name: "wrong prefix", arg: "X-a", wantByte: 0, wantOK: false},
		{name: "empty string", arg: "", wantByte: 0, wantOK: false},
		{name: "too long", arg: "C-aa", wantByte: 0, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotByte, gotOK := parseControlKey(tt.arg)
			if gotByte != tt.wantByte || gotOK != tt.wantOK {
				t.Fatalf("parseControlKey(%q) = (0x%02X, %v), want (0x%02X, %v)",
					tt.arg, gotByte, gotOK, tt.wantByte, tt.wantOK)
			}
		})
	}
}

func TestTranslateSendKeysEmptyInput(t *testing.T) {
	got := TranslateSendKeys([]string{})
	if got != nil {
		t.Fatalf("TranslateSendKeys([]string{}) = %q, want nil", got)
	}

	got = TranslateSendKeys(nil)
	if got != nil {
		t.Fatalf("TranslateSendKeys(nil) = %q, want nil", got)
	}
}

func TestTranslateSendKeysAllSpecialKeys(t *testing.T) {
	// Exhaustive test for every key in sendKeysTable.
	// Keys use their canonical display casing (e.g. "Enter", "KPEnter") rather than
	// the all-lowercase table keys, verifying that normalizeSendKeyToken correctly
	// resolves case-insensitive input to the table entry.
	tests := []struct {
		name string
		key  string
		want []byte
	}{
		{name: "Enter", key: "Enter", want: []byte{'\r'}},
		{name: "KPEnter", key: "KPEnter", want: []byte{'\r'}},
		{name: "C-c", key: "C-c", want: []byte{0x03}},
		{name: "C-d", key: "C-d", want: []byte{0x04}},
		{name: "C-z", key: "C-z", want: []byte{0x1a}},
		{name: "C-[", key: "C-[", want: []byte{0x1b}},
		{name: "Escape", key: "Escape", want: []byte{0x1b}},
		{name: "Space", key: "Space", want: []byte{' '}},
		{name: "Tab", key: "Tab", want: []byte{'\t'}},
		{name: "BSpace", key: "BSpace", want: []byte{0x7f}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TranslateSendKeys([]string{tt.key})
			if string(got) != string(tt.want) {
				t.Fatalf("TranslateSendKeys([%q]) = %q (0x%02X), want %q (0x%02X)",
					tt.key, got, got, tt.want, tt.want)
			}
		})
	}
}

func TestParseControlKeyFullAlphabetLowercase(t *testing.T) {
	// C-a through C-z should produce 0x01 through 0x1a.
	for ch := byte('a'); ch <= byte('z'); ch++ {
		arg := "C-" + string(ch)
		wantByte := ch - 'a' + 1
		t.Run(arg, func(t *testing.T) {
			gotByte, gotOK := parseControlKey(arg)
			if !gotOK {
				t.Fatalf("parseControlKey(%q) returned false, want true", arg)
			}
			if gotByte != wantByte {
				t.Fatalf("parseControlKey(%q) = 0x%02X, want 0x%02X", arg, gotByte, wantByte)
			}
		})
	}
}

func TestParseControlKeyFullAlphabetUppercase(t *testing.T) {
	// C-A through C-Z should produce 0x01 through 0x1a (same as lowercase).
	for ch := byte('A'); ch <= byte('Z'); ch++ {
		arg := "C-" + string(ch)
		wantByte := ch - 'A' + 1
		t.Run(arg, func(t *testing.T) {
			gotByte, gotOK := parseControlKey(arg)
			if !gotOK {
				t.Fatalf("parseControlKey(%q) returned false, want true", arg)
			}
			if gotByte != wantByte {
				t.Fatalf("parseControlKey(%q) = 0x%02X, want 0x%02X", arg, gotByte, wantByte)
			}
		})
	}
}

func TestParseControlKeyAllSpecialCharacters(t *testing.T) {
	// All non-alphabetic control characters handled by parseControlKey.
	tests := []struct {
		name     string
		arg      string
		wantByte byte
	}{
		{name: "C-@ (NUL)", arg: "C-@", wantByte: 0x00},
		{name: `C-\ (FS)`, arg: "C-\\", wantByte: 0x1c},
		{name: "C-] (GS)", arg: "C-]", wantByte: 0x1d},
		{name: "C-^ (RS)", arg: "C-^", wantByte: 0x1e},
		{name: "C-_ (US)", arg: "C-_", wantByte: 0x1f},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotByte, gotOK := parseControlKey(tt.arg)
			if !gotOK {
				t.Fatalf("parseControlKey(%q) returned false, want true", tt.arg)
			}
			if gotByte != tt.wantByte {
				t.Fatalf("parseControlKey(%q) = 0x%02X, want 0x%02X", tt.arg, gotByte, tt.wantByte)
			}
		})
	}
}

func TestParseControlKeyInvalidInputs(t *testing.T) {
	tests := []struct {
		name string
		arg  string
	}{
		{name: "empty string", arg: ""},
		{name: "single char", arg: "C"},
		{name: "prefix only", arg: "C-"},
		{name: "too long", arg: "C-ab"},
		{name: "wrong prefix X", arg: "X-a"},
		{name: "wrong prefix M", arg: "M-a"},
		{name: "digit", arg: "C-1"},
		{name: "punctuation not in table", arg: "C-!"},
		{name: "space char", arg: "C- "},
		{name: "C-[ handled by table not parseControlKey", arg: "C-["},
		{name: "just a letter", arg: "a"},
		{name: "full word", arg: "ctrl-a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, gotOK := parseControlKey(tt.arg)
			if gotOK {
				t.Fatalf("parseControlKey(%q) returned true, want false", tt.arg)
			}
		})
	}
}

func TestTranslateSendKeysTableOverridesParseControlKey(t *testing.T) {
	// Keys in sendKeysTable should be resolved by the table, not by parseControlKey.
	// For example, "C-c" is in the table and should produce 0x03.
	// parseControlKey("C-c") would also produce 0x03, but the table path is preferred.
	// This test verifies behavioral correctness regardless of resolution path.
	tableKeys := map[string][]byte{
		"C-c": {0x03},
		"C-d": {0x04},
		"C-z": {0x1a},
		"C-[": {0x1b},
	}

	for key, wantBytes := range tableKeys {
		t.Run("table-"+key, func(t *testing.T) {
			got := TranslateSendKeys([]string{key})
			if string(got) != string(wantBytes) {
				t.Fatalf("TranslateSendKeys([%q]) = 0x%02X, want 0x%02X", key, got, wantBytes)
			}
		})
	}
}

func TestTranslateSendKeysUnknownKeyPassedThrough(t *testing.T) {
	// Keys not in the table and not matching C-{letter} should be passed as-is.
	tests := []struct {
		name string
		args []string
		want []byte
	}{
		{name: "plain text", args: []string{"hello"}, want: []byte("hello")},
		{name: "multiple plain args", args: []string{"abc", "def"}, want: []byte("abcdef")},
		{name: "single char", args: []string{"x"}, want: []byte("x")},
		{name: "number", args: []string{"123"}, want: []byte("123")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TranslateSendKeys(tt.args)
			if string(got) != string(tt.want) {
				t.Fatalf("TranslateSendKeys(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestTranslateSendKeysLowercaseControlKeyIntegration(t *testing.T) {
	// TC-3: verify normalizeSendKeyToken → table miss → parseControlKey integration
	// for lowercase control key tokens not in sendKeysTable.
	tests := []struct {
		name string
		arg  string
		want []byte
	}{
		{name: "c-m (lowercase) via parseControlKey", arg: "c-m", want: []byte{0x0D}},
		{name: "c-a (lowercase) via parseControlKey", arg: "c-a", want: []byte{0x01}},
		{name: "c-l (lowercase) via parseControlKey", arg: "c-l", want: []byte{0x0C}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TranslateSendKeys([]string{tt.arg})
			if string(got) != string(tt.want) {
				t.Fatalf("TranslateSendKeys([%q]) = %q (0x%02X), want %q (0x%02X)",
					tt.arg, got, got, tt.want, tt.want)
			}
		})
	}
}

func TestTranslateSendKeysNormalizesBeforeParseControlKey(t *testing.T) {
	// C-1 regression: parseControlKey must receive the normalized (trimmed+lowered) arg.
	// Before the fix, " C-m" would fail parseControlKey (len=4) and be passed through as raw bytes.
	tests := []struct {
		name string
		arg  string
		want []byte
	}{
		{name: "leading space C-m", arg: " C-m", want: []byte{0x0D}},
		{name: "trailing space C-a", arg: "C-a ", want: []byte{0x01}},
		{name: "both spaces C-l", arg: " C-l ", want: []byte{0x0C}},
		{name: "uppercase with space C-M", arg: " C-M ", want: []byte{0x0D}},
		{name: "space around table key C-c", arg: " C-c ", want: []byte{0x03}},
		{name: "space around Enter", arg: " Enter ", want: []byte{'\r'}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TranslateSendKeys([]string{tt.arg})
			if string(got) != string(tt.want) {
				t.Fatalf("TranslateSendKeys([%q]) = %q (0x%02X), want %q (0x%02X)",
					tt.arg, got, got, tt.want, tt.want)
			}
		})
	}
}

func TestTranslateSendKeysAllKeysCaseInsensitive(t *testing.T) {
	// Verify normalizeSendKeyToken handles case variants for all non-Enter/KPEnter keys.
	// Enter/KPEnter are covered by TestTranslateSendKeysEnterAliasesCaseInsensitive.
	tests := []struct {
		name string
		key  string
		want []byte
	}{
		{name: "Space mixed case", key: "Space", want: []byte{' '}},
		{name: "space lowercase", key: "space", want: []byte{' '}},
		{name: "SPACE uppercase", key: "SPACE", want: []byte{' '}},
		{name: "Tab mixed case", key: "Tab", want: []byte{'\t'}},
		{name: "tab lowercase", key: "tab", want: []byte{'\t'}},
		{name: "TAB uppercase", key: "TAB", want: []byte{'\t'}},
		{name: "Escape mixed case", key: "Escape", want: []byte{0x1b}},
		{name: "escape lowercase", key: "escape", want: []byte{0x1b}},
		{name: "ESCAPE uppercase", key: "ESCAPE", want: []byte{0x1b}},
		{name: "BSpace mixed case", key: "BSpace", want: []byte{0x7f}},
		{name: "bspace lowercase", key: "bspace", want: []byte{0x7f}},
		{name: "BSPACE uppercase", key: "BSPACE", want: []byte{0x7f}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TranslateSendKeys([]string{tt.key})
			if string(got) != string(tt.want) {
				t.Fatalf("TranslateSendKeys([%q]) = %q (0x%02X), want %q (0x%02X)",
					tt.key, got, got, tt.want, tt.want)
			}
		})
	}
}

func TestTranslateSendKeysComplexSequences(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []byte
	}{
		{
			name: "command with Enter",
			args: []string{"ls", "Space", "-la", "Enter"},
			want: []byte("ls -la\r"),
		},
		{
			name: "ctrl-c after text",
			args: []string{"running", "C-c"},
			want: []byte("running\x03"),
		},
		{
			name: "tab completion",
			args: []string{"cd", "Space", "/ho", "Tab"},
			want: []byte("cd /ho\t"),
		},
		{
			name: "escape sequence",
			args: []string{"Escape", "[", "A"},
			want: []byte("\x1b[A"),
		},
		{
			name: "backspace then retype",
			args: []string{"abc", "BSpace", "d"},
			want: []byte("abc\x7fd"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TranslateSendKeys(tt.args)
			if string(got) != string(tt.want) {
				t.Fatalf("TranslateSendKeys(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestCopyModeCommandTableAllEntries(t *testing.T) {
	// T-4: Exhaustive byte-level verification for every entry in copyModeCommandTable.
	tests := []struct {
		name    string
		command string
		want    []byte
	}{
		{name: "cancel", command: "cancel", want: []byte{0x1b}},
		{name: "page-up", command: "page-up", want: []byte{0x1b, '[', '5', '~'}},
		{name: "halfpage-up", command: "halfpage-up", want: []byte{0x1b, '[', '5', '~'}},
		{name: "page-down", command: "page-down", want: []byte{0x1b, '[', '6', '~'}},
		{name: "halfpage-down", command: "halfpage-down", want: []byte{0x1b, '[', '6', '~'}},
		{name: "cursor-up", command: "cursor-up", want: []byte{0x1b, '[', 'A'}},
		{name: "cursor-down", command: "cursor-down", want: []byte{0x1b, '[', 'B'}},
		{name: "cursor-right", command: "cursor-right", want: []byte{0x1b, '[', 'C'}},
		{name: "cursor-left", command: "cursor-left", want: []byte{0x1b, '[', 'D'}},
		{name: "start-of-line", command: "start-of-line", want: []byte{0x1b, '[', 'H'}},
		{name: "end-of-line", command: "end-of-line", want: []byte{0x1b, '[', 'F'}},
		{name: "history-top", command: "history-top", want: []byte{0x1b, '[', '1', ';', '5', 'H'}},
		{name: "history-bottom", command: "history-bottom", want: []byte{0x1b, '[', '1', ';', '5', 'F'}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := TranslateCopyModeCommand(tt.command)
			if !ok {
				t.Fatalf("TranslateCopyModeCommand(%q) returned false", tt.command)
			}
			if string(got) != string(tt.want) {
				t.Fatalf("TranslateCopyModeCommand(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}

	// Verify we tested all entries in the table.
	if len(tests) != len(copyModeCommandTable) {
		t.Fatalf("test count = %d, copyModeCommandTable size = %d; add missing entries", len(tests), len(copyModeCommandTable))
	}
}

func TestTranslateCopyModeCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    []byte
		wantOK  bool
	}{
		{
			name:    "cancel returns Escape",
			command: "cancel",
			want:    []byte{0x1b},
			wantOK:  true,
		},
		{
			name:    "Cancel is case-insensitive",
			command: "Cancel",
			want:    []byte{0x1b},
			wantOK:  true,
		},
		{
			name:    "CANCEL uppercase",
			command: "CANCEL",
			want:    []byte{0x1b},
			wantOK:  true,
		},
		{
			name:    "cancel with whitespace is trimmed",
			command: "  cancel  ",
			want:    []byte{0x1b},
			wantOK:  true,
		},
		{
			name:    "page-up returns Page Up sequence",
			command: "page-up",
			want:    []byte{0x1b, '[', '5', '~'},
			wantOK:  true,
		},
		{
			name:    "cursor-up returns Up arrow",
			command: "cursor-up",
			want:    []byte{0x1b, '[', 'A'},
			wantOK:  true,
		},
		{
			name:    "unknown command returns false",
			command: "select-word",
			want:    nil,
			wantOK:  false,
		},
		{
			name:    "empty command returns false",
			command: "",
			want:    nil,
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := TranslateCopyModeCommand(tt.command)
			if ok != tt.wantOK {
				t.Fatalf("TranslateCopyModeCommand(%q) ok = %v, want %v", tt.command, ok, tt.wantOK)
			}
			if string(got) != string(tt.want) {
				t.Fatalf("TranslateCopyModeCommand(%q) = %q, want %q", tt.command, got, tt.want)
			}
		})
	}
}
