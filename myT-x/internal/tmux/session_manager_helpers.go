package tmux

import (
	"fmt"
	"maps"
	"strconv"
	"strings"
)

// copyBoolPtr returns a shallow copy of a *bool pointer.
// Returns nil when src is nil.
func copyBoolPtr(src *bool) *bool {
	if src == nil {
		return nil
	}
	v := *src
	return &v
}

func copyEnvMap(input map[string]string) map[string]string {
	// Preserve caller safety by always returning a mutable map:
	// nil/empty input -> empty non-nil map.
	if len(input) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(input))
	maps.Copy(out, input)
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
