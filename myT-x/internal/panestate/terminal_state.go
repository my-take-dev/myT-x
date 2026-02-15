package panestate

import (
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"
)

// maxCSILen is the maximum number of runes consumed inside a single CSI
// sequence. Beyond this limit the parser forcibly resets to prevent a
// malformed or adversarial input from silently suppressing all output.
const maxCSILen = 256

type escapeMode uint8

const (
	escapeNone escapeMode = iota
	escapeInitial
	escapeCSI
	escapeOSC
)

type terminalState struct {
	cols int
	rows int

	lines [][]rune
	head  int // physical index of logical row 0 (ring buffer rotation point)
	row   int
	col   int // col can be == cols temporarily; wrapping occurs on the next putRune

	escapeMode    escapeMode
	oscEscPending bool
	csiLen        int // number of runes consumed in current CSI sequence

	remainder [utf8.UTFMax]byte // buffer for incomplete multi-byte sequence at chunk boundary
	remLen    int               // valid bytes in remainder
}

func newTerminalState(cols int, rows int) *terminalState {
	cols, rows = sanitizeSize(cols, rows)
	lines := make([][]rune, rows)
	for i := range lines {
		lines[i] = make([]rune, 0, cols)
	}
	return &terminalState{
		cols:  cols,
		rows:  rows,
		lines: lines,
		head:  0,
	}
}

// physIdx converts a logical row index to a physical index in the ring buffer.
func (t *terminalState) physIdx(logicalRow int) int {
	return (t.head + logicalRow) % len(t.lines)
}

// Size returns the current terminal dimensions.
func (t *terminalState) Size() (int, int) {
	return t.cols, t.rows
}

func (t *terminalState) Resize(cols int, rows int) {
	cols, rows = sanitizeSize(cols, rows)

	// Reset any in-flight escape sequence to avoid stale parser state
	// carrying over after a terminal size change.
	t.resetEscape()

	if rows != t.rows {
		// Linearize ring buffer into logical order before reshaping.
		oldRows := t.rows
		if oldRows > len(t.lines) {
			oldRows = len(t.lines)
		}
		linearized := make([][]rune, oldRows)
		for i := 0; i < oldRows; i++ {
			linearized[i] = t.lines[t.physIdx(i)]
		}

		newLines := make([][]rune, rows)
		if rows > oldRows {
			// Growing: copy existing lines, append blank rows.
			copy(newLines, linearized)
			for i := oldRows; i < rows; i++ {
				newLines[i] = make([]rune, 0, cols)
			}
		} else {
			// Shrinking: keep the tail (most recent) rows.
			start := 0
			if len(linearized) > rows {
				start = len(linearized) - rows
			}
			copy(newLines, linearized[start:])
		}
		t.lines = newLines
		t.head = 0 // linearized, reset ring pointer
	}

	// Truncate lines wider than new cols.
	for i := range t.lines {
		if len(t.lines[i]) > cols {
			t.lines[i] = t.lines[i][:cols]
		}
	}

	t.cols = cols
	t.rows = rows

	// Clamp cursor to the new grid. After a shrink the cursor may reference
	// a row/col that no longer exists.
	if t.col > t.cols {
		t.col = t.cols
	}
	if t.row >= t.rows {
		t.row = t.rows - 1
	}
	if t.row < 0 {
		t.row = 0
	}
}

// Write processes chunk through the terminal emulator. The returned error is
// always nil; the signature satisfies io.Writer for composability.
func (t *terminalState) Write(chunk []byte) (int, error) {
	n := len(chunk)

	// Prepend any remainder from the previous chunk boundary.
	if t.remLen > 0 {
		need := utf8NeedBytes(t.remainder[0]) - t.remLen
		if need > len(chunk) {
			// Still not enough bytes to complete the sequence.
			copy(t.remainder[t.remLen:], chunk)
			t.remLen += len(chunk)
			return n, nil
		}
		copy(t.remainder[t.remLen:], chunk[:need])
		r, _ := utf8.DecodeRune(t.remainder[:t.remLen+need])
		t.consumeRune(r)
		chunk = chunk[need:]
		t.remLen = 0
	}

	for len(chunk) > 0 {
		b := chunk[0]
		// Fast path: ASCII (single byte)
		if b < utf8.RuneSelf {
			t.consumeRune(rune(b))
			chunk = chunk[1:]
			continue
		}

		r, size := utf8.DecodeRune(chunk)
		if r == utf8.RuneError && size == 1 {
			if !utf8.FullRune(chunk) {
				// Incomplete multi-byte at end of chunk; buffer it.
				t.remLen = copy(t.remainder[:], chunk)
				break
			}
			// Truly invalid byte; skip it.
			slog.Debug("[panestate] skipping invalid UTF-8 byte", "byte", fmt.Sprintf("0x%02X", b))
			chunk = chunk[1:]
			continue
		}
		t.consumeRune(r)
		chunk = chunk[size:]
	}
	return n, nil
}

// utf8NeedBytes returns the expected byte count for a UTF-8 sequence
// starting with the given leading byte.
func utf8NeedBytes(b byte) int {
	switch {
	case b < 0x80:
		return 1
	case b < 0xE0:
		return 2
	case b < 0xF0:
		return 3
	default:
		return 4
	}
}

func (t *terminalState) String() string {
	if t.rows == 0 {
		return ""
	}

	var b strings.Builder
	b.Grow(t.rows * (t.cols + 1))
	for i := 0; i < t.rows; i++ {
		line := t.lines[t.physIdx(i)]
		if len(line) > 0 {
			b.WriteString(string(line))
		}
		if i+1 < t.rows {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func (t *terminalState) consumeRune(r rune) {
	if t.escapeMode != escapeNone {
		t.consumeEscapeRune(r)
		return
	}

	switch r {
	case 0x1b:
		t.escapeMode = escapeInitial
	case '\r':
		t.col = 0
	case '\n':
		t.newLine()
	case '\b':
		if t.col > 0 {
			t.col--
		}
	case '\t':
		spaces := 8 - (t.col % 8)
		for i := 0; i < spaces; i++ {
			t.putRune(' ')
		}
	default:
		if r < 0x20 || r == 0x7f {
			return
		}
		t.putRune(r)
	}
}

func (t *terminalState) consumeEscapeRune(r rune) {
	switch t.escapeMode {
	case escapeInitial:
		switch r {
		case '[':
			t.escapeMode = escapeCSI
			t.csiLen = 0
		case ']':
			t.escapeMode = escapeOSC
			t.oscEscPending = false
		default:
			t.resetEscape()
		}
	case escapeCSI:
		t.csiLen++
		// A CSI sequence ends when a final-byte rune [@-~] appears.
		// CR/LF also forcibly terminates a malformed sequence.
		if r >= 0x40 && r <= 0x7e {
			t.resetEscape()
		} else if r == '\r' || r == '\n' {
			t.resetEscape()
		} else if t.csiLen >= maxCSILen {
			slog.Warn("[panestate] DEBUG CSI sequence exceeded max length, resetting parser", "csiLen", t.csiLen)
			t.resetEscape()
		}
	case escapeOSC:
		if r == 0x07 {
			t.resetEscape()
			return
		}
		if t.oscEscPending && r == '\\' {
			t.resetEscape()
			return
		}
		t.oscEscPending = (r == 0x1b)
		if r == '\r' || r == '\n' {
			t.resetEscape()
		}
	default:
		t.resetEscape()
	}
}

func (t *terminalState) resetEscape() {
	t.escapeMode = escapeNone
	t.oscEscPending = false
	t.csiLen = 0
}

func (t *terminalState) putRune(r rune) {
	if t.cols <= 0 || t.rows <= 0 {
		slog.Warn("[panestate] DEBUG putRune called with zero dimensions, dropping rune",
			"cols", t.cols, "rows", t.rows)
		return
	}
	if t.row >= t.rows {
		t.row = t.rows - 1
	}
	if t.col >= t.cols {
		t.newLine()
	}

	idx := t.physIdx(t.row)
	line := t.lines[idx]
	for len(line) < t.col {
		line = append(line, ' ')
	}
	if len(line) == t.col {
		line = append(line, r)
	} else {
		line[t.col] = r
	}
	if len(line) > t.cols {
		line = line[:t.cols]
	}
	t.lines[idx] = line
	t.col++
}

func (t *terminalState) newLine() {
	if t.rows <= 0 {
		return
	}
	if t.row < t.rows-1 {
		t.row++
		t.col = 0
		return
	}

	// Safety net: if lines is somehow empty, re-initialize.
	if len(t.lines) == 0 {
		slog.Warn("[panestate] DEBUG newLine: lines is empty, re-initializing", "rows", t.rows)
		t.lines = make([][]rune, t.rows)
		for i := range t.lines {
			t.lines[i] = make([]rune, 0, t.cols)
		}
		t.head = 0
	}

	// O(1) scroll: advance head and clear the recycled line.
	oldHead := t.head
	t.head = (t.head + 1) % len(t.lines)
	t.lines[oldHead] = t.lines[oldHead][:0] // reuse backing array
	t.col = 0
}

// sanitizeSize ensures positive terminal dimensions, substituting defaults
// for zero or negative values.
func sanitizeSize(cols int, rows int) (int, int) {
	if cols <= 0 {
		cols = defaultCols
	}
	if rows <= 0 {
		rows = defaultRows
	}
	return cols, rows
}
