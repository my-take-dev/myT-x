package mcptool

import (
	"fmt"
	"regexp"
	"strings"

	"myT-x/internal/mcp/agent-orchestrator/domain"
	"myT-x/internal/mcp/agent-orchestrator/usecase"
)

const (
	maxAgentNameLen     = 64
	maxRoleLen          = 120
	maxSkillLen         = 100
	maxSkillDescLen     = 400
	maxSkills           = 20
	maxMessageLen       = 8000
	maxTaskIDLen        = 64
	maxCaptureLines     = 200
	maxPaneTitleLen     = 30
	maxCommandLen       = 100
	maxCustomMessageLen = 2000
	maxTeamNameLen      = 64
	maxArgs             = 20
	maxArgLen           = 200
	maxStatusNoteLen    = 200
	maxCancelReasonLen  = 500
	maxProgressNoteLen  = 500
	maxGroupLabelLen    = 120
	maxBatchTasks       = 10
	maxDependsOnTasks   = 20
)

var (
	agentNamePattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	taskIDPattern        = regexp.MustCompile(`^t-[A-Za-z0-9]+$`)
	sendMessageIDPattern = regexp.MustCompile(`^m-[A-Za-z0-9]+$`)
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

func optionalTaskID(args map[string]any, key string) (string, error) {
	value, ok := args[key]
	if !ok || value == nil {
		return "", nil
	}
	str, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	if strings.TrimSpace(str) == "" {
		return "", nil
	}
	return validateTaskIDString(key, str)
}

func validateTaskIDString(key string, value string) (string, error) {
	if !taskIDPattern.MatchString(value) {
		return "", fmt.Errorf("%s must match %s", key, taskIDPattern.String())
	}
	return value, nil
}

func requiredSendMessageID(args map[string]any, key string) (string, error) {
	value, err := requiredString(args, key, maxTaskIDLen)
	if err != nil {
		return "", err
	}
	if !sendMessageIDPattern.MatchString(value) {
		return "", fmt.Errorf("%s must match %s", key, sendMessageIDPattern.String())
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

func optionalBoundedStringPtr(args map[string]any, key string, maxLen int) (*string, error) {
	value, ok := args[key]
	if !ok || value == nil {
		return nil, nil
	}
	str, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("%s must be a string", key)
	}
	if len([]rune(str)) > maxLen {
		return nil, fmt.Errorf("%s must be %d characters or fewer", key, maxLen)
	}
	return &str, nil
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

func optionalTaskIDList(args map[string]any, key string, maxItems int) ([]string, error) {
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

	taskIDs := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for i, item := range raw {
		str, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("%s[%d] must be a string", key, i)
		}
		taskID, err := validateTaskIDString(fmt.Sprintf("%s[%d]", key, i), str)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[taskID]; exists {
			return nil, fmt.Errorf("%s[%d] duplicates %q", key, i, taskID)
		}
		seen[taskID] = struct{}{}
		taskIDs = append(taskIDs, taskID)
	}
	return taskIDs, nil
}

func requiredBatchTasks(args map[string]any, key string) ([]usecase.SendTaskBatchItemCmd, error) {
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
	if len(raw) > maxBatchTasks {
		return nil, fmt.Errorf("%s must contain %d items or fewer", key, maxBatchTasks)
	}

	items := make([]usecase.SendTaskBatchItemCmd, 0, len(raw))
	for i, item := range raw {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s[%d] must be an object", key, i)
		}
		agentName, err := requiredAgentName(entry, "agent_name")
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", key, i, err)
		}
		message, err := requiredMessage(entry, "message")
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", key, i, err)
		}
		includeInstructions, err := optionalBool(entry, "include_response_instructions", true)
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", key, i, err)
		}
		expiresAfterMinutes, err := optionalIntBounded(entry, "expires_after_minutes", 0, 1, 1440)
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", key, i, err)
		}
		items = append(items, usecase.SendTaskBatchItemCmd{
			AgentName:                   agentName,
			Message:                     message,
			IncludeResponseInstructions: includeInstructions,
			ExpiresAfterMinutes:         expiresAfterMinutes,
		})
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
	case "all", "pending", "blocked", "completed", "failed", "abandoned", "cancelled", "expired":
		return status, nil
	default:
		return "", fmt.Errorf("%s must be one of all, pending, blocked, completed, failed, abandoned, cancelled, expired", key)
	}
}

func requiredAgentWorkStatus(args map[string]any, key string) (string, error) {
	value, err := requiredString(args, key, 16)
	if err != nil {
		return "", err
	}
	switch value {
	case domain.AgentWorkStatusIdle, domain.AgentWorkStatusBusy, domain.AgentWorkStatusWorking:
		return value, nil
	default:
		return "", fmt.Errorf("%s must be one of idle, busy, working", key)
	}
}

// optionalPaneID は任意のペインIDを取得する。未指定時は空文字を返す。
func optionalPaneID(args map[string]any, key string) (string, error) {
	value, ok := args[key]
	if !ok || value == nil {
		return "", nil
	}
	str, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	if strings.TrimSpace(str) == "" {
		return "", nil
	}
	if err := domain.ValidatePaneID(str); err != nil {
		return "", err
	}
	return str, nil
}

// optionalIntBounded は任意の整数パラメータを範囲付きで取得する。
func optionalIntBounded(args map[string]any, key string, defaultValue, minVal, maxVal int) (int, error) {
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
	n := int(number)
	if n < minVal || n > maxVal {
		return 0, fmt.Errorf("%s must be between %d and %d", key, minVal, maxVal)
	}
	return n, nil
}

func optionalProgressPct(args map[string]any, key string) (*int, error) {
	value, ok := args[key]
	if !ok || value == nil {
		return nil, nil
	}
	number, ok := value.(float64)
	if !ok {
		return nil, fmt.Errorf("%s must be a number", key)
	}
	if number != float64(int(number)) {
		return nil, fmt.Errorf("%s must be an integer", key)
	}
	progressPct := int(number)
	if progressPct < 0 || progressPct > 100 {
		return nil, fmt.Errorf("%s must be between 0 and 100", key)
	}
	return &progressPct, nil
}

// optionalSplitDirection は分割方向を取得する。"horizontal" または "vertical"。
func optionalSplitDirection(args map[string]any, key string, defaultValue string) (string, error) {
	value, ok := args[key]
	if !ok || value == nil {
		return defaultValue, nil
	}
	str, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	switch str {
	case "horizontal", "vertical":
		return str, nil
	default:
		return "", fmt.Errorf("%s must be one of horizontal, vertical", key)
	}
}
