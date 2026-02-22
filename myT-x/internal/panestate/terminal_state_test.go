package panestate

import (
	"strings"
	"testing"
)

func TestTerminalStateWrite(t *testing.T) {
	tests := []struct {
		name  string
		cols  int
		rows  int
		input string
		want  string
	}{
		{
			name:  "simple text",
			cols:  20,
			rows:  2,
			input: "hello",
			want:  "hello\n",
		},
		{
			name:  "ANSI SGR stripped",
			cols:  40,
			rows:  4,
			input: "\x1b[31mred\x1b[0m normal",
			want:  "red normal\n\n\n",
		},
		{
			name:  "256-color CSI stripped",
			cols:  20,
			rows:  2,
			input: "\x1b[38;5;196mcolor\x1b[0m",
			want:  "color\n",
		},
		{
			name:  "CSI cursor movement stripped",
			cols:  20,
			rows:  2,
			input: "\x1b[2Jcleared",
			want:  "cleared\n",
		},
		{
			name:  "scroll keeps tail rows",
			cols:  12,
			rows:  2,
			input: "line1\nline2\nline3",
			want:  "line2\nline3",
		},
		{
			name:  "line wrapping at column boundary",
			cols:  5,
			rows:  3,
			input: "abcdefgh",
			want:  "abcde\nfgh\n",
		},
		{
			name:  "carriage return overwrites",
			cols:  10,
			rows:  2,
			input: "AAAA\rBB",
			want:  "BBAA\n",
		},
		{
			name:  "backspace moves cursor back",
			cols:  10,
			rows:  2,
			input: "abc\b\bXY",
			want:  "aXY\n",
		},
		{
			name:  "backspace at column 0 stays",
			cols:  10,
			rows:  2,
			input: "\b\bhi",
			want:  "hi\n",
		},
		{
			name:  "tab stops at 8-column boundaries",
			cols:  20,
			rows:  2,
			input: "a\tb",
			want:  "a       b\n",
		},
		{
			name:  "control chars filtered",
			cols:  10,
			rows:  2,
			input: "a\x01\x02\x7fb",
			want:  "ab\n",
		},
		{
			name:  "OSC terminated by BEL",
			cols:  20,
			rows:  2,
			input: "\x1b]0;title\x07visible",
			want:  "visible\n",
		},
		{
			name:  "OSC terminated by ST (ESC backslash)",
			cols:  20,
			rows:  2,
			input: "\x1b]0;title\x1b\\visible",
			want:  "visible\n",
		},
		// CSI interrupted by newline
		{
			name:  "CSI interrupted by newline",
			cols:  20,
			rows:  3,
			input: "\x1b[31m\ntext",
			want:  "\ntext\n",
		},
		// Bare ESC followed by non-CSI/OSC
		{
			name:  "bare ESC with unknown char",
			cols:  20,
			rows:  2,
			input: "\x1b?text",
			want:  "text\n",
		},
		// Back-to-back escape sequences
		{
			name:  "back-to-back CSI sequences",
			cols:  20,
			rows:  2,
			input: "\x1b[31m\x1b[0mtext",
			want:  "text\n",
		},
		// Unicode: multi-byte UTF-8 characters (CJK)
		// Note: CJK characters are treated as single-width; this is a known
		// limitation documented by this test.
		{
			name:  "CJK characters stored as runes",
			cols:  10,
			rows:  2,
			input: "日本語",
			want:  "日本語\n",
		},
		// Wrapping that triggers scroll
		{
			name:  "wrap plus scroll",
			cols:  3,
			rows:  2,
			input: "abcdefghi",
			want:  "def\nghi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := newTerminalState(tt.cols, tt.rows)
			_, _ = term.Write([]byte(tt.input))
			got := term.String()
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTerminalStateWriteSplitMultiByte(t *testing.T) {
	tests := []struct {
		name   string
		chunks [][]byte
		want   string
	}{
		{
			name:   "CJK 3-byte split after 1st byte",
			chunks: [][]byte{[]byte("A\xe6"), []byte("\x97\xa5B")}, // A + 日 + B
			want:   "A日B\n",
		},
		{
			name:   "CJK 3-byte split after 2nd byte",
			chunks: [][]byte{[]byte("A\xe6\x97"), []byte("\xa5B")}, // A + 日 + B
			want:   "A日B\n",
		},
		{
			name:   "Emoji 4-byte split after 1st byte",
			chunks: [][]byte{[]byte("\xf0"), []byte("\x9f\x98\x80end")}, // 😀 + end
			want:   "😀end\n",
		},
		{
			name:   "Emoji 4-byte split after 2nd byte",
			chunks: [][]byte{[]byte("X\xf0\x9f"), []byte("\x98\x80")}, // X + 😀
			want:   "X😀\n",
		},
		{
			name:   "Emoji 4-byte split after 3rd byte",
			chunks: [][]byte{[]byte("\xf0\x9f\x98"), []byte("\x80Y")}, // 😀 + Y
			want:   "😀Y\n",
		},
		{
			name:   "ASCII only, no split needed",
			chunks: [][]byte{[]byte("hello"), []byte(" world")},
			want:   "hello world\n",
		},
		{
			name:   "Invalid UTF-8 byte skipped",
			chunks: [][]byte{[]byte("A\xffB")},
			want:   "AB\n",
		},
		{
			name:   "Multiple CJK across chunk boundary",
			chunks: [][]byte{[]byte("\xe6\x97"), []byte("\xa5\xe6\x9c\xac\xe8\xaa"), []byte("\x9e")}, // 日本語
			want:   "日本語\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := newTerminalState(20, 2)
			for _, chunk := range tt.chunks {
				_, _ = term.Write(chunk)
			}
			got := term.String()
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTerminalStateWriteReturnValue(t *testing.T) {
	term := newTerminalState(20, 2)
	input := []byte("hello world")
	n, err := term.Write(input)
	if n != len(input) {
		t.Errorf("Write returned n=%d, want %d", n, len(input))
	}
	if err != nil {
		t.Errorf("Write returned error: %v", err)
	}
}

func TestTerminalStateResize(t *testing.T) {
	t.Run("grow rows preserves content", func(t *testing.T) {
		term := newTerminalState(10, 2)
		_, _ = term.Write([]byte("AAA\nBBB"))
		term.Resize(10, 4)
		got := term.String()
		if !strings.Contains(got, "AAA") || !strings.Contains(got, "BBB") {
			t.Errorf("content lost after grow: %q", got)
		}
	})

	t.Run("shrink rows keeps latest lines", func(t *testing.T) {
		term := newTerminalState(10, 4)
		_, _ = term.Write([]byte("L1\nL2\nL3\nL4"))
		term.Resize(10, 2)
		got := term.String()
		if strings.Contains(got, "L1") || strings.Contains(got, "L2") {
			t.Errorf("old lines should be gone: %q", got)
		}
		if !strings.Contains(got, "L3") || !strings.Contains(got, "L4") {
			t.Errorf("latest lines should be kept: %q", got)
		}
	})

	t.Run("shrink columns truncates lines", func(t *testing.T) {
		term := newTerminalState(10, 2)
		_, _ = term.Write([]byte("1234567890"))
		term.Resize(5, 2)
		got := term.String()
		if strings.Contains(got, "67890") {
			t.Errorf("line should be truncated to 5 cols: %q", got)
		}
		if !strings.Contains(got, "12345") {
			t.Errorf("first 5 chars should remain: %q", got)
		}
	})

	t.Run("write after resize works", func(t *testing.T) {
		term := newTerminalState(10, 4)
		_, _ = term.Write([]byte("before"))
		term.Resize(10, 2)
		_, _ = term.Write([]byte("\nafter"))
		got := term.String()
		if !strings.Contains(got, "after") {
			t.Errorf("write after resize should work: %q", got)
		}
	})

	t.Run("resize to same dimensions is noop", func(t *testing.T) {
		term := newTerminalState(10, 3)
		_, _ = term.Write([]byte("ABC\nDEF"))
		before := term.String()
		term.Resize(10, 3)
		after := term.String()
		if before != after {
			t.Errorf("resize to same dims changed content: before=%q after=%q", before, after)
		}
	})

	t.Run("cursor clamped after shrink below cursor row", func(t *testing.T) {
		term := newTerminalState(10, 5)
		_, _ = term.Write([]byte("L1\nL2\nL3\nL4\nL5"))
		// Cursor is at row 4 (last row). Shrink to 2 rows.
		term.Resize(10, 2)
		// Write after shrink should not panic.
		_, _ = term.Write([]byte("\nNEW"))
		got := term.String()
		if !strings.Contains(got, "NEW") {
			t.Errorf("write after cursor-clamping shrink should work: %q", got)
		}
	})

	t.Run("cursor clamped after column shrink", func(t *testing.T) {
		term := newTerminalState(10, 2)
		_, _ = term.Write([]byte("1234567890"))
		// Cursor is at col 10. Shrink cols to 5.
		term.Resize(5, 2)
		// Write after shrink should not panic and should appear correctly.
		_, _ = term.Write([]byte("X"))
		got := term.String()
		if !strings.Contains(got, "X") {
			t.Errorf("write after col shrink should produce output: %q", got)
		}
	})

	t.Run("cols-only shrink with head != 0", func(t *testing.T) {
		term := newTerminalState(10, 3)
		// Write enough to scroll (head != 0).
		_, _ = term.Write([]byte("AAAAAAAAAA\nBBBBBBBBBB\nCCCCCCCCCC\nDDDDDDDDDD"))
		// Shrink cols only (rows unchanged) — no linearization.
		term.Resize(5, 3)
		got := term.String()
		if !strings.Contains(got, "BBBBB") || !strings.Contains(got, "CCCCC") || !strings.Contains(got, "DDDDD") {
			t.Errorf("cols-only resize with scrolled head should truncate correctly: %q", got)
		}
		// Verify write still works.
		_, _ = term.Write([]byte("\nEE"))
		got2 := term.String()
		if !strings.Contains(got2, "EE") {
			t.Errorf("write after cols-only resize failed: %q", got2)
		}
	})

	t.Run("multiple sequential resizes", func(t *testing.T) {
		term := newTerminalState(10, 4)
		_, _ = term.Write([]byte("AB\nCD"))
		term.Resize(20, 6) // grow
		_, _ = term.Write([]byte("\nEF"))
		term.Resize(5, 2) // shrink
		_, _ = term.Write([]byte("\nGH"))
		got := term.String()
		if !strings.Contains(got, "GH") {
			t.Errorf("final write should appear after sequential resizes: %q", got)
		}
	})
}

func TestTerminalStateStringFormat(t *testing.T) {
	term := newTerminalState(5, 3)
	_, _ = term.Write([]byte("AB\nCD"))
	got := term.String()
	// 3 rows: "AB", "CD", "" joined by \n => "AB\nCD\n"
	want := "AB\nCD\n"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestRingBufferScrollPreservesContent(t *testing.T) {
	tests := []struct {
		name  string
		cols  int
		rows  int
		input string
		want  string
	}{
		{
			name:  "exactly fills then scrolls once",
			cols:  10,
			rows:  3,
			input: "L1\nL2\nL3\nL4",
			want:  "L2\nL3\nL4",
		},
		{
			name:  "heavy scrolling wraps head multiple times",
			cols:  5,
			rows:  2,
			input: "A\nB\nC\nD\nE\nF",
			want:  "E\nF",
		},
		{
			name:  "single row terminal",
			cols:  10,
			rows:  1,
			input: "first\nsecond\nthird",
			want:  "third",
		},
		{
			name:  "many scrolls preserve correct tail",
			cols:  4,
			rows:  3,
			input: "1\n2\n3\n4\n5\n6\n7\n8\n9\n10",
			want:  "8\n9\n10",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := newTerminalState(tt.cols, tt.rows)
			_, _ = term.Write([]byte(tt.input))
			got := term.String()
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRingBufferResizeAfterScroll(t *testing.T) {
	tests := []struct {
		name       string
		cols, rows int
		input      string
		newRows    int
		wantParts  []string
	}{
		{
			name:      "shrink after scroll keeps tail",
			cols:      10,
			rows:      3,
			input:     "A\nB\nC\nD\nE",
			newRows:   2,
			wantParts: []string{"D", "E"},
		},
		{
			name:      "grow after scroll preserves all",
			cols:      10,
			rows:      2,
			input:     "X\nY\nZ",
			newRows:   4,
			wantParts: []string{"Y", "Z"},
		},
		{
			name:      "write after resize+scroll works",
			cols:      10,
			rows:      3,
			input:     "1\n2\n3\n4\n5",
			newRows:   2,
			wantParts: []string{"4", "5"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := newTerminalState(tt.cols, tt.rows)
			_, _ = term.Write([]byte(tt.input))
			term.Resize(tt.cols, tt.newRows)
			got := term.String()
			for _, part := range tt.wantParts {
				if !strings.Contains(got, part) {
					t.Errorf("got %q, missing %q", got, part)
				}
			}

			// Verify write after resize still works.
			_, _ = term.Write([]byte("\nPOST"))
			got2 := term.String()
			if !strings.Contains(got2, "POST") {
				t.Errorf("write after resize failed: %q", got2)
			}
		})
	}
}

func BenchmarkNewLineScroll(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		term := newTerminalState(120, 40)
		// Fill terminal then trigger 1000 scrolls.
		for range 40 + 1000 {
			term.newLine()
		}
	}
}

func BenchmarkTerminalString(b *testing.B) {
	term := newTerminalState(120, 40)
	chunk := make([]byte, 120*40)
	for i := range chunk {
		if i%121 == 120 {
			chunk[i] = '\n'
		} else {
			chunk[i] = 'A' + byte(i%26)
		}
	}
	_, _ = term.Write(chunk)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = term.String()
	}
}

func BenchmarkTerminalWrite(b *testing.B) {
	// 120 cols x 40 rows, typical terminal output (ASCII with newlines)
	chunk := make([]byte, 32*1024)
	for i := range chunk {
		if i%121 == 120 {
			chunk[i] = '\n'
		} else {
			chunk[i] = 'A' + byte(i%26)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		term := newTerminalState(120, 40)
		_, _ = term.Write(chunk)
	}
}

func BenchmarkPutRuneFullLine(b *testing.B) {
	// Fill a 120-column line character by character
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		term := newTerminalState(120, 2)
		for range 120 {
			term.putRune('X')
		}
	}
}

func TestTerminalStateSizeAccessor(t *testing.T) {
	term := newTerminalState(80, 24)
	cols, rows := term.Size()
	if cols != 80 || rows != 24 {
		t.Errorf("Size() = (%d, %d), want (80, 24)", cols, rows)
	}
	term.Resize(40, 10)
	cols, rows = term.Size()
	if cols != 40 || rows != 10 {
		t.Errorf("Size() after resize = (%d, %d), want (40, 10)", cols, rows)
	}
}
