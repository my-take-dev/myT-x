package tmux

import (
	"fmt"
	"strconv"
	"strings"
)

func copyEnvMap(input map[string]string) map[string]string {
	// Preserve caller safety by always returning a mutable map:
	// nil/empty input -> empty non-nil map.
	if len(input) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

// ParseCallerPane parses a TMUX_PANE-like id string.
func ParseCallerPane(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return -1
	}
	id, err := parsePaneID(value)
	if err != nil {
		return -1
	}
	return id
}

func parsePaneID(value string) (int, error) {
	if !strings.HasPrefix(value, "%") {
		return -1, fmt.Errorf("invalid pane id: %s", value)
	}
	id, err := strconv.Atoi(strings.TrimPrefix(value, "%"))
	if err != nil || id < 0 {
		return -1, fmt.Errorf("invalid pane id: %s", value)
	}
	return id, nil
}
