package domain

import (
	"fmt"
	"regexp"
	"strings"
)

// TaskStatus はタスクの状態を表す defined type。
type TaskStatus string

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

// IsTerminal returns true when the task has reached a final state.
func (status TaskStatus) IsTerminal() bool {
	switch status {
	case TaskStatusCompleted, TaskStatusFailed, TaskStatusAbandoned, TaskStatusCancelled, TaskStatusExpired:
		return true
	default:
		return false
	}
}

// TaskStatusFilter represents a task-list filter value.
// It is intentionally separate from TaskStatus so sentinel values such as
// "all" do not leak into the task lifecycle enum.
type TaskStatusFilter string

const (
	TaskStatusFilterAll       TaskStatusFilter = "all"
	TaskStatusFilterPending                    = TaskStatusFilter(TaskStatusPending)
	TaskStatusFilterBlocked                    = TaskStatusFilter(TaskStatusBlocked)
	TaskStatusFilterCompleted                  = TaskStatusFilter(TaskStatusCompleted)
	TaskStatusFilterFailed                     = TaskStatusFilter(TaskStatusFailed)
	TaskStatusFilterAbandoned                  = TaskStatusFilter(TaskStatusAbandoned)
	TaskStatusFilterCancelled                  = TaskStatusFilter(TaskStatusCancelled)
	TaskStatusFilterExpired                    = TaskStatusFilter(TaskStatusExpired)
)

var taskStatusFilterValues = []TaskStatusFilter{
	TaskStatusFilterAll,
	TaskStatusFilterPending,
	TaskStatusFilterBlocked,
	TaskStatusFilterCompleted,
	TaskStatusFilterFailed,
	TaskStatusFilterAbandoned,
	TaskStatusFilterCancelled,
	TaskStatusFilterExpired,
}

func IsValidTaskStatusFilter(value string) bool {
	switch TaskStatusFilter(value) {
	case TaskStatusFilterAll,
		TaskStatusFilterPending,
		TaskStatusFilterBlocked,
		TaskStatusFilterCompleted,
		TaskStatusFilterFailed,
		TaskStatusFilterAbandoned,
		TaskStatusFilterCancelled,
		TaskStatusFilterExpired:
		return true
	default:
		return false
	}
}

func TaskStatusFilterValues() []string {
	values := make([]string, 0, len(taskStatusFilterValues))
	for _, value := range taskStatusFilterValues {
		values = append(values, string(value))
	}
	return values
}

func (filter TaskStatusFilter) MatchesTaskStatus(status TaskStatus) bool {
	switch filter {
	case "", TaskStatusFilterAll:
		return true
	case TaskStatusFilterPending:
		return status == TaskStatusPending
	case TaskStatusFilterBlocked:
		return status == TaskStatusBlocked
	case TaskStatusFilterCompleted:
		return status == TaskStatusCompleted
	case TaskStatusFilterFailed:
		return status == TaskStatusFailed
	case TaskStatusFilterAbandoned:
		return status == TaskStatusAbandoned
	case TaskStatusFilterCancelled:
		return status == TaskStatusCancelled
	case TaskStatusFilterExpired:
		return status == TaskStatusExpired
	default:
		return false
	}
}

// TaskStatusFilterFromString returns the canonical filter constant for a value
// already validated by IsValidTaskStatusFilter. Unknown values are preserved so
// legacy direct callers fail closed instead of silently broadening queries.
func TaskStatusFilterFromString(value string) TaskStatusFilter {
	switch TaskStatusFilter(value) {
	case TaskStatusFilterAll:
		return TaskStatusFilterAll
	case TaskStatusFilterPending:
		return TaskStatusFilterPending
	case TaskStatusFilterBlocked:
		return TaskStatusFilterBlocked
	case TaskStatusFilterCompleted:
		return TaskStatusFilterCompleted
	case TaskStatusFilterFailed:
		return TaskStatusFilterFailed
	case TaskStatusFilterAbandoned:
		return TaskStatusFilterAbandoned
	case TaskStatusFilterCancelled:
		return TaskStatusFilterCancelled
	case TaskStatusFilterExpired:
		return TaskStatusFilterExpired
	default:
		return TaskStatusFilter(value)
	}
}

// AgentWorkStatus represents the current activity state of an agent.
type AgentWorkStatus string

const (
	// AgentWorkStatusUnknown is the read-only fallback returned when the current
	// agent status cannot be determined after registration reconciliation.
	// External callers must not set this value.
	AgentWorkStatusUnknown AgentWorkStatus = "unknown"
	AgentWorkStatusIdle    AgentWorkStatus = "idle"
	// AgentWorkStatusBusy means the agent cannot accept interactive work right now
	// because it is occupied by non-task activity or external constraints.
	AgentWorkStatusBusy AgentWorkStatus = "busy"
	// AgentWorkStatusWorking means the agent is actively processing an assigned
	// orchestrator task and can report task-linked progress or completion.
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
	AgentName     string          `json:"agent_name"`
	Status        AgentWorkStatus `json:"status"`
	CurrentTaskID string          `json:"current_task_id,omitempty"`
	Note          string          `json:"note,omitempty"`
	UpdatedAt     string          `json:"updated_at,omitempty"`
}

// TaskGroup stores metadata for a batch of related tasks.
type TaskGroup struct {
	ID        string `json:"group_id"`
	Label     string `json:"label,omitempty"`
	CreatedAt string `json:"created_at"`
}

// Task はタスク情報を表す。
type Task struct {
	ID               string     `json:"task_id"`
	AgentName        string     `json:"agent_name"`
	AssigneePaneID   string     `json:"assignee_pane_id,omitempty"`
	SenderPaneID     string     `json:"sender_pane_id,omitempty"`
	SenderName       string     `json:"sender_name,omitempty"`
	SenderInstanceID string     `json:"sender_instance_id,omitempty"`
	SendMessageID    string     `json:"send_message_id,omitempty"`
	SendResponseID   string     `json:"send_response_id,omitempty"`
	Status           TaskStatus `json:"status"`
	SentAt           string     `json:"sent_at"`
	// CompletedAt stores the terminal timestamp for any finished task state,
	// including completed, cancelled, expired, failed, and abandoned.
	CompletedAt    string `json:"completed_at,omitempty"`
	AcknowledgedAt string `json:"acknowledged_at,omitempty"`
	// CancelledAt stores the dedicated cancellation timestamp for cancelled
	// tasks. CompletedAt still records the terminal timestamp for that state.
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
	Status       TaskStatusFilter
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
