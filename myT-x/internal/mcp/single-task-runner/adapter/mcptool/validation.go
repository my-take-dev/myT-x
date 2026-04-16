package mcptool

import (
	"fmt"
	"strings"

	"myT-x/internal/singletaskrunner"
)

func requiredBoundedString(args map[string]any, key string, maxLen int) (string, error) {
	value, ok := args[key]
	if !ok || value == nil {
		return "", fmt.Errorf("%s is required", key)
	}
	str, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	str = strings.TrimSpace(str)
	if str == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	if len([]rune(str)) > maxLen {
		return "", fmt.Errorf("%s must be %d characters or fewer", key, maxLen)
	}
	return str, nil
}

func optionalBoundedString(args map[string]any, key string, maxLen int) (string, error) {
	value, ok := args[key]
	if !ok || value == nil {
		return "", nil
	}
	str, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	str = strings.TrimSpace(str)
	if len([]rune(str)) > maxLen {
		return "", fmt.Errorf("%s must be %d characters or fewer", key, maxLen)
	}
	return str, nil
}

func optionalBool(args map[string]any, key string, defaultValue bool) (bool, error) {
	value, ok := args[key]
	if !ok || value == nil {
		return defaultValue, nil
	}
	boolean, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("%s must be a boolean", key)
	}
	return boolean, nil
}

func requiredTaskID(args map[string]any, key string) (string, error) {
	return requiredBoundedString(args, key, 200)
}

func requiredEnqueueTasks(args map[string]any, key string, maxItems int) ([]singletaskrunner.EnqueueTaskInput, error) {
	value, ok := args[key]
	if !ok || value == nil {
		return nil, fmt.Errorf("%s is required", key)
	}
	raw, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array", key)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("%s must contain at least 1 item", key)
	}
	if len(raw) > maxItems {
		return nil, fmt.Errorf("%s must contain %d items or fewer", key, maxItems)
	}

	tasks := make([]singletaskrunner.EnqueueTaskInput, 0, len(raw))
	for i, item := range raw {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s[%d] must be an object", key, i)
		}

		message, err := requiredBoundedString(entry, "message", 8000)
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", key, i, err)
		}
		title, err := optionalBoundedString(entry, "title", 200)
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", key, i, err)
		}
		if title == "" {
			title = message
			if len([]rune(title)) > 200 {
				title = string([]rune(title)[:200])
			}
		}
		clearBefore, err := optionalBool(entry, "clear_before", false)
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", key, i, err)
		}
		clearCommand, err := optionalBoundedString(entry, "clear_command", 200)
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", key, i, err)
		}

		tasks = append(tasks, singletaskrunner.EnqueueTaskInput{
			Title:        title,
			Message:      message,
			ClearBefore:  clearBefore,
			ClearCommand: clearCommand,
		})
	}

	return tasks, nil
}
