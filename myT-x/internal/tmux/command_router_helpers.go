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

func resolvePercentageDimension(reference int, percent int) (int, bool) {
	if reference <= 0 || percent <= 0 {
		return 0, false
	}
	maxInt := int(^uint(0) >> 1)
	if reference > maxInt/percent {
		return 0, false
	}
	return reference * percent / 100, true
}

func resolveResizeDimension(value any, reference int, defaultValue int, flag string) (int, error) {
	if value == nil {
		return defaultValue, nil
	}

	switch v := value.(type) {
	case int:
		if v <= 0 {
			return 0, fmt.Errorf("flag %s must be positive", flag)
		}
		return v, nil
	case float64:
		size := int(v)
		if size <= 0 {
			return 0, fmt.Errorf("flag %s must be positive", flag)
		}
		return size, nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0, fmt.Errorf("flag %s requires a value", flag)
		}
		if strings.HasSuffix(trimmed, "%") {
			if reference <= 0 {
				return 0, fmt.Errorf("flag %s cannot use percentage without a positive reference size", flag)
			}
			percentText := strings.TrimSpace(strings.TrimSuffix(trimmed, "%"))
			percent, err := strconv.Atoi(percentText)
			if err != nil || percent <= 0 {
				return 0, fmt.Errorf("flag %s expects a positive integer or percentage, got %q", flag, v)
			}
			computed, ok := resolvePercentageDimension(reference, percent)
			if !ok {
				return 0, fmt.Errorf("flag %s percentage overflow: %d%% of %d", flag, percent, reference)
			}
			if computed <= 0 {
				computed = 1 // clamp to minimum cell size
			}
			return computed, nil
		}
		size, err := strconv.Atoi(trimmed)
		if err != nil || size <= 0 {
			return 0, fmt.Errorf("flag %s expects a positive integer or percentage, got %q", flag, v)
		}
		return size, nil
	default:
		return 0, fmt.Errorf("flag %s expects a positive integer or percentage", flag)
	}
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
