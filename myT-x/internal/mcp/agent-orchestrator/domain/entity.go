package domain

import (
	"fmt"
	"regexp"
)

// TaskStatus はタスクの状態を表す列挙型。
type TaskStatus = string

// TaskStatus の定数定義。
const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusAbandoned TaskStatus = "abandoned"
)

var paneIDPattern = regexp.MustCompile(`^%[0-9]+$`)

// ValidatePaneID は tmux ペインIDを検証する。
func ValidatePaneID(paneID string) error {
	if paneID == "" {
		return fmt.Errorf("pane_id is required")
	}
	if !paneIDPattern.MatchString(paneID) {
		return fmt.Errorf("invalid pane_id %q: must match ^%%[0-9]+$", paneID)
	}
	return nil
}

// Skill はエージェントの得意分野を表す。
type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Agent はエージェント登録情報を表す。
type Agent struct {
	Name          string  `json:"name"`
	PaneID        string  `json:"pane_id"`
	Role          string  `json:"role,omitempty"`
	Skills        []Skill `json:"skills,omitempty"`
	CreatedAt     string  `json:"created_at,omitempty"`
	MCPInstanceID string  `json:"mcp_instance_id,omitempty"`
}

// Task はタスク情報を表す。
type Task struct {
	ID               string `json:"task_id"`
	AgentName        string `json:"agent_name"`
	AssigneePaneID   string `json:"assignee_pane_id,omitempty"`
	SenderPaneID     string `json:"sender_pane_id,omitempty"`
	SenderName       string `json:"sender_name,omitempty"`
	SenderInstanceID string `json:"sender_instance_id,omitempty"`
	SendMessageID    string `json:"send_message_id,omitempty"`
	SendResponseID   string `json:"send_response_id,omitempty"`
	Status           string `json:"status"`
	SentAt           string `json:"sent_at"`
	CompletedAt      string `json:"completed_at,omitempty"`
	IsNowSession     bool   `json:"is_now_session"`
}

// TaskMessage はタスクメッセージを表す。
type TaskMessage struct {
	ID        string `json:"id"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

// TaskFilter はタスク検索のフィルタ条件を表す。
type TaskFilter struct {
	Status       string
	AgentName    string
	IsNowSession *bool // nil=フィルタなし, true/false でフィルタ
}

// PaneInfo は tmux ペインの情報を表す。
type PaneInfo struct {
	ID      string
	Title   string
	Session string
	Window  string
}
