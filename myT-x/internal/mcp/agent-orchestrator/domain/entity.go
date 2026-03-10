package domain

import (
	"fmt"
	"regexp"
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

// Agent はエージェント登録情報を表す。
type Agent struct {
	Name      string   `json:"name"`
	PaneID    string   `json:"pane_id"`
	Role      string   `json:"role,omitempty"`
	Skills    []string `json:"skills,omitempty"`
	CreatedAt string   `json:"created_at,omitempty"`
}

// Task はタスク情報を表す。
type Task struct {
	ID             string `json:"task_id"`
	AgentName      string `json:"agent_name"`
	AssigneePaneID string `json:"assignee_pane_id,omitempty"`
	SenderName     string `json:"sender_name,omitempty"`
	Label          string `json:"label,omitempty"`
	Status         string `json:"status"`
	SentAt         string `json:"sent_at"`
	CompletedAt    string `json:"completed_at,omitempty"`
	Notes          string `json:"notes,omitempty"`
}

// TaskFilter はタスク検索のフィルタ条件を表す。
type TaskFilter struct {
	Status    string
	AgentName string
}

// PaneInfo は tmux ペインの情報を表す。
type PaneInfo struct {
	ID      string
	Title   string
	Session string
	Window  string
}
