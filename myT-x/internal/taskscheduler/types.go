package taskscheduler

// QueueItemStatus はキューアイテムの状態を表す。
type QueueItemStatus = string

const (
	ItemStatusPending   QueueItemStatus = "pending"
	ItemStatusRunning   QueueItemStatus = "running"
	ItemStatusCompleted QueueItemStatus = "completed"
	ItemStatusFailed    QueueItemStatus = "failed"
	ItemStatusSkipped   QueueItemStatus = "skipped"
)

// IsEditable returns true if the item can be edited or deleted.
func IsEditable(status QueueItemStatus) bool {
	switch status {
	case ItemStatusPending, ItemStatusCompleted, ItemStatusFailed, ItemStatusSkipped:
		return true
	default:
		return false
	}
}

// IsTerminal returns true when the item has already reached a final state.
func IsTerminal(status QueueItemStatus) bool {
	switch status {
	case ItemStatusCompleted, ItemStatusFailed, ItemStatusSkipped:
		return true
	default:
		return false
	}
}

// QueueRunStatus はキュー全体の実行状態を表す。
type QueueRunStatus = string

const (
	QueueIdle      QueueRunStatus = "idle"
	QueuePreparing QueueRunStatus = "preparing"
	QueueRunning   QueueRunStatus = "running"
	QueuePaused    QueueRunStatus = "paused"
	QueueCompleted QueueRunStatus = "completed"
)

// QueueItem はキュー内の個別タスクを表す。
type QueueItem struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Message      string `json:"message"`
	TargetPaneID string `json:"target_pane_id"`
	OrderIndex   int    `json:"order_index"`
	Status       string `json:"status"`
	OrcTaskID    string `json:"orc_task_id,omitempty"`
	CreatedAt    string `json:"created_at"`
	StartedAt    string `json:"started_at,omitempty"`
	CompletedAt  string `json:"completed_at,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
	ClearBefore  bool   `json:"clear_before"`
	ClearCommand string `json:"clear_command,omitempty"`
}

type PreExecTargetMode = string

const (
	PreExecTargetModeAllPanes  PreExecTargetMode = "all_panes"
	PreExecTargetModeTaskPanes PreExecTargetMode = "task_panes"
)

// QueueConfig はキュー実行設定。
type QueueConfig struct {
	PreExecEnabled     bool              `json:"pre_exec_enabled"`
	PreExecTargetMode  PreExecTargetMode `json:"pre_exec_target_mode"`
	PreExecResetDelay  int               `json:"pre_exec_reset_delay_s"`
	PreExecIdleTimeout int               `json:"pre_exec_idle_timeout_s"`
}

func applyConfigDefaults(c *QueueConfig) {
	if c == nil {
		return
	}
	if c.PreExecResetDelay < 0 {
		c.PreExecResetDelay = 10
	}
	if c.PreExecIdleTimeout <= 0 {
		c.PreExecIdleTimeout = 120
	}
	if c.PreExecTargetMode == "" {
		c.PreExecTargetMode = PreExecTargetModeTaskPanes
	}
}

// QueueStatus はキューの現在状態をフロントエンドに返す DTO。
type QueueStatus struct {
	Config          QueueConfig `json:"config"`
	Items           []QueueItem `json:"items"`
	RunStatus       string      `json:"run_status"`
	CurrentIndex    int         `json:"current_index"`
	SessionName     string      `json:"session_name"`
	PreExecProgress string      `json:"pre_exec_progress,omitempty"`
}
