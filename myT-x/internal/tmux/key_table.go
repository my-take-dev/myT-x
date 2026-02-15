package tmux

var sendKeysTable = map[string][]byte{
	"Enter":  {'\r'},
	"C-c":    {0x03},
	"C-d":    {0x04},
	"C-z":    {0x1a},
	"Escape": {0x1b},
	"Space":  {' '},
	"Tab":    {'\t'},
	"BSpace": {0x7f},
}

// TranslateSendKeys translates tmux send-keys arguments to bytes.
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
		out = append(out, arg...)
	}
	return out
}
