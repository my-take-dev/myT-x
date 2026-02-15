package tmux

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"myT-x/internal/ipc"
)

func mustBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		b, err := strconv.ParseBool(v)
		if err != nil {
			slog.Debug("[DEBUG-ROUTER] mustBool: failed to parse string value",
				"value", v,
				"error", err,
			)
			return false
		}
		return b
	case float64:
		return v != 0
	case int:
		return v != 0
	default:
		return false
	}
}

func mustInt(value any, defaultValue int) int {
	switch v := value.(type) {
	case int:
		return v
	case float64:
		return int(v)
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(v))
		if err == nil {
			return i
		}
	}
	return defaultValue
}

func mustString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case int:
		return strconv.Itoa(v)
	case float64:
		return strconv.Itoa(int(v))
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func okResp(stdout string) ipc.TmuxResponse {
	return ipc.TmuxResponse{
		ExitCode: 0,
		Stdout:   stdout,
	}
}

func errResp(err error) ipc.TmuxResponse {
	return ipc.TmuxResponse{
		ExitCode: 1,
		Stderr:   fmt.Sprintf("%v\n", err),
	}
}

// truncateBytes returns a printable preview of the first n bytes.
func truncateBytes(data []byte, maxLen int) string {
	if len(data) <= maxLen {
		return fmt.Sprintf("%q", string(data))
	}
	return fmt.Sprintf("%q...(+%d bytes)", string(data[:maxLen]), len(data)-maxLen)
}
