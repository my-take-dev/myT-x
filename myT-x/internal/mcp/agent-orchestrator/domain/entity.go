package domain

import (
	"fmt"
	"regexp"
	"strings"
)

// TaskStatus はタスクの状態を表す列挙型。
type TaskStatus = string

// TaskStatus の定数定義。
const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusBlocked   TaskStatus = "blocked"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusAbandoned TaskStatus = "abandoned"
	TaskStatusCancelled TaskStatus = "cancelled"
	TaskStatusExpired   TaskStatus = "expired"
)

// AgentWorkStatus represents the current activity state of an agent.
type AgentWorkStatus = string

const (
	AgentWorkStatusUnknown AgentWorkStatus = "unknown"
	AgentWorkStatusIdle    AgentWorkStatus = "idle"
	AgentWorkStatusBusy    AgentWorkStatus = "busy"
	AgentWorkStatusWorking AgentWorkStatus = "working"
)

// VirtualPaneIDPrefix is the prefix for virtual pane IDs that do not
// correspond to real tmux panes. Virtual panes are used by internal
// services (e.g. task scheduler) that participate in the orchestrator
// task protocol without owning a physical terminal pane.
const VirtualPaneIDPrefix = "%virtual-"

// IsVirtualPaneID reports whether paneID represents a virtual (non-tmux) pane.
func IsVirtualPaneID(paneID string) bool {
	return strings.HasPrefix(paneID, VirtualPaneIDPrefix)
}

var paneIDPattern = regexp.MustCompile(`^%[0-9]+$`)

// ValidatePaneID は tmux ペインIDまたは仮想ペインIDを検証する。
func ValidatePaneID(paneID string) error {
	if paneID == "" {
		return fmt.Errorf("pane_id is required")
	}
	if IsVirtualPaneID(paneID) {
		return nil
	}
	if !paneIDPattern.MatchString(paneID) {
		return fmt.Errorf("invalid pane_id %q: must match ^%%[0-9]+$ or be a virtual pane id", paneID)
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

// AgentStatus stores the last reported work status for an agent.
type AgentStatus struct {
	AgentName     string `json:"agent_name"`
	Status        string `json:"status"`
	CurrentTaskID string `json:"current_task_id,omitempty"`
	Note          string `json:"note,omitempty"`
	UpdatedAt     string `json:"updated_at,omitempty"`
}

// TaskGroup stores metadata for a batch of related tasks.
type TaskGroup struct {
	ID        string `json:"group_id"`
	Label     string `json:"label,omitempty"`
	CreatedAt string `json:"created_at"`
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
	// CompletedAt stores the terminal timestamp for any finished task state,
	// including completed, cancelled, expired, failed, and abandoned.
	CompletedAt       string `json:"completed_at,omitempty"`
	AcknowledgedAt    string `json:"acknowledged_at,omitempty"`
	CancelledAt       string `json:"cancelled_at,omitempty"`
	CancelReason      string `json:"cancel_reason,omitempty"`
	ProgressPct       *int   `json:"progress_pct,omitempty"`
	ProgressNote      string `json:"progress_note,omitempty"`
	ProgressUpdatedAt string `json:"progress_updated_at,omitempty"`
	ExpiresAt         string `json:"expires_at,omitempty"`
	GroupID           string `json:"group_id,omitempty"`
	IsNowSession      bool   `json:"is_now_session"`
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
