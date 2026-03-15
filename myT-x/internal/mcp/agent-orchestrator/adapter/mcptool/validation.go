package mcptool

import (
	"fmt"
	"regexp"
	"strings"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

const (
	maxAgentNameLen = 64
	maxRoleLen      = 120
	maxSkillLen     = 100
	maxSkillDescLen = 400
	maxSkills       = 20
	maxMessageLen   = 8000
	maxTaskIDLen    = 64
	maxCaptureLines = 200
)

var (
	agentNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	taskIDPattern    = regexp.MustCompile(`^t-[A-Za-z0-9]+$`)
)

func requiredAgentName(args map[string]any, key string) (string, error) {
	value, err := requiredString(args, key, maxAgentNameLen)
	if err != nil {
		return "", err
	}
	return validateAgentNameString(key, value)
}

func validateAgentNameString(key string, value string) (string, error) {
	if !agentNamePattern.MatchString(value) {
		return "", fmt.Errorf("%s must match %s", key, agentNamePattern.String())
	}
	return value, nil
}

func optionalAgentName(args map[string]any, key string) (string, error) {
	value, ok := args[key]
	if !ok || value == nil {
		return "", nil
	}
	str, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	if str == "" {
		return "", nil
	}
	return validateAgentNameString(key, str)
}

func requiredTaskID(args map[string]any, key string) (string, error) {
	value, err := requiredString(args, key, maxTaskIDLen)
	if err != nil {
		return "", err
	}
	return validateTaskIDString(key, value)
}

func validateTaskIDString(key string, value string) (string, error) {
	if !taskIDPattern.MatchString(value) {
		return "", fmt.Errorf("%s must match %s", key, taskIDPattern.String())
	}
	return value, nil
}

func requiredPaneID(args map[string]any, key string) (string, error) {
	value, err := requiredString(args, key, 16)
	if err != nil {
		return "", err
	}
	if err := domain.ValidatePaneID(value); err != nil {
		return "", err
	}
	return value, nil
}

func requiredMessage(args map[string]any, key string) (string, error) {
	return requiredString(args, key, maxMessageLen)
}

func requiredString(args map[string]any, key string, maxLen int) (string, error) {
	value, ok := args[key]
	if !ok {
		return "", fmt.Errorf("%s is required", key)
	}
	str, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	if strings.TrimSpace(str) == "" {
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
	if len([]rune(str)) > maxLen {
		return "", fmt.Errorf("%s must be %d characters or fewer", key, maxLen)
	}
	return str, nil
}

func optionalStringList(args map[string]any, key string, maxItems int, maxItemLen int) ([]string, error) {
	value, ok := args[key]
	if !ok || value == nil {
		return nil, nil
	}
	raw, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array", key)
	}
	if len(raw) > maxItems {
		return nil, fmt.Errorf("%s must contain %d items or fewer", key, maxItems)
	}

	items := make([]string, 0, len(raw))
	for i, item := range raw {
		str, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("%s[%d] must be a string", key, i)
		}
		if strings.TrimSpace(str) == "" {
			return nil, fmt.Errorf("%s[%d] is required", key, i)
		}
		if len([]rune(str)) > maxItemLen {
			return nil, fmt.Errorf("%s[%d] must be %d characters or fewer", key, i, maxItemLen)
		}
		items = append(items, str)
	}
	return items, nil
}

// optionalSkillList はスキル配列をパースする。
// オブジェクト配列 [{"name":"x","description":"y"}] とレガシー文字列配列 ["x"] の両方を受け付ける。
func optionalSkillList(args map[string]any, key string, maxItems int) ([]domain.Skill, error) {
	value, ok := args[key]
	if !ok || value == nil {
		return nil, nil
	}
	raw, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array", key)
	}
	if len(raw) > maxItems {
		return nil, fmt.Errorf("%s must contain %d items or fewer", key, maxItems)
	}

	skills := make([]domain.Skill, 0, len(raw))
	for i, item := range raw {
		switch v := item.(type) {
		case string:
			// レガシー文字列形式
			if strings.TrimSpace(v) == "" {
				return nil, fmt.Errorf("%s[%d] name is required", key, i)
			}
			if len([]rune(v)) > maxSkillLen {
				return nil, fmt.Errorf("%s[%d] name must be %d characters or fewer", key, i, maxSkillLen)
			}
			skills = append(skills, domain.Skill{Name: v})
		case map[string]any:
			// オブジェクト形式
			name, nameOK := v["name"].(string)
			if !nameOK || strings.TrimSpace(name) == "" {
				return nil, fmt.Errorf("%s[%d] name is required", key, i)
			}
			if len([]rune(name)) > maxSkillLen {
				return nil, fmt.Errorf("%s[%d] name must be %d characters or fewer", key, i, maxSkillLen)
			}
			desc, _ := v["description"].(string)
			if len([]rune(desc)) > maxSkillDescLen {
				return nil, fmt.Errorf("%s[%d] description must be %d characters or fewer", key, i, maxSkillDescLen)
			}
			skills = append(skills, domain.Skill{Name: name, Description: desc})
		default:
			return nil, fmt.Errorf("%s[%d] must be a string or object", key, i)
		}
	}
	return skills, nil
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

func optionalLines(args map[string]any, key string, defaultValue int, maxValue int) (int, error) {
	value, ok := args[key]
	if !ok || value == nil {
		return defaultValue, nil
	}
	number, ok := value.(float64)
	if !ok {
		return 0, fmt.Errorf("%s must be a number", key)
	}
	if number != float64(int(number)) {
		return 0, fmt.Errorf("%s must be an integer", key)
	}
	lines := int(number)
	if lines < 1 || lines > maxValue {
		return 0, fmt.Errorf("%s must be between 1 and %d", key, maxValue)
	}
	return lines, nil
}

func optionalStatusFilter(args map[string]any, key string, defaultValue string) (string, error) {
	value, ok := args[key]
	if !ok || value == nil {
		return defaultValue, nil
	}
	status, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	switch status {
	case "all", "pending", "completed", "failed", "abandoned":
		return status, nil
	default:
		return "", fmt.Errorf("%s must be one of all, pending, completed, failed, abandoned", key)
	}
}
