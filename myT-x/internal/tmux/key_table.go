package tmux

// sendKeysTable maps named key literals to their byte sequences.
//
// S-42: The C-c, C-d, C-z, C-[ entries intentionally duplicate coverage
// provided by parseControlKey. They exist in the table for two reasons:
//  1. Table lookup is O(1) and avoids the parseControlKey function call overhead.
//  2. They serve as documentation of the most commonly used control sequences.
//
// parseControlKey acts as a fallback for C-{letter} patterns not explicitly
// listed here (C-a through C-z, plus special chars @, \, ], ^, _).
var sendKeysTable = map[string][]byte{
	"Enter":  {'\r'},
	"C-c":    {0x03},
	"C-d":    {0x04},
	"C-z":    {0x1a},
	"C-[":    {0x1b},
	"Escape": {0x1b},
	"Space":  {' '},
	"Tab":    {'\t'},
	"BSpace": {0x7f},
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
		if value, ok := sendKeysTable[arg]; ok {
			out = append(out, value...)
			continue
		}
		if b, ok := parseControlKey(arg); ok {
			out = append(out, b)
			continue
		}
		out = append(out, arg...)
	}
	return out
}

// parseControlKey parses "C-{letter}" notation into a control byte.
// C-a = 0x01, C-b = 0x02, ..., C-z = 0x1a.
// Returns (byte, true) on success.
func parseControlKey(arg string) (byte, bool) {
	if len(arg) != 3 || arg[0] != 'C' || arg[1] != '-' {
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
