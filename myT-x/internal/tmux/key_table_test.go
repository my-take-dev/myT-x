package tmux

import "testing"

func TestTranslateSendKeys(t *testing.T) {
	got := TranslateSendKeys([]string{"echo", "Space", "ok", "Enter"})
	want := []byte("echo ok\r")
	if string(got) != string(want) {
		t.Fatalf("TranslateSendKeys() = %q, want %q", string(got), string(want))
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
	tests := []struct {
		name string
		key  string
		want []byte
	}{
		{name: "Enter", key: "Enter", want: []byte{'\r'}},
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
