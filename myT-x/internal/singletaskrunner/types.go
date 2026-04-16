package singletaskrunner

// QueueItemStatus represents the execution status of one queued task.
type QueueItemStatus string

const (
	ItemStatusPending   QueueItemStatus = "pending"
	ItemStatusSending   QueueItemStatus = "sending"
	ItemStatusActive    QueueItemStatus = "active"
	ItemStatusDone      QueueItemStatus = "done"
	ItemStatusFailed    QueueItemStatus = "failed"
	ItemStatusCancelled QueueItemStatus = "cancelled"
)

// IsEditable returns true when the item can be edited or deleted.
func (status QueueItemStatus) IsEditable() bool {
	switch status {
	case ItemStatusPending, ItemStatusDone, ItemStatusFailed, ItemStatusCancelled:
		return true
	default:
		return false
	}
}

// IsTerminal returns true when the item reached a final state.
func (status QueueItemStatus) IsTerminal() bool {
	switch status {
	case ItemStatusDone, ItemStatusFailed, ItemStatusCancelled:
		return true
	default:
		return false
	}
}

// QueueRunStatus represents the queue-wide execution state.
// There is no dedicated "failed" state. When execution stops because of a
// failure or manual interruption, the queue returns to QueueIdle and records the
// latest stop reason in QueueStatus.LastStopReason.
type QueueRunStatus string

const (
	QueueIdle      QueueRunStatus = "idle"
	QueueRunning   QueueRunStatus = "running"
	QueueCompleted QueueRunStatus = "completed"
)

const (
	DefaultClearCommand = "/new"
	DefaultClearDelay   = 2
	MinClearDelaySec    = 0
	MaxClearDelaySec    = 300
)

// QueueItem is the frontend-safe DTO for one queued task.
type QueueItem struct {
	ID            string          `json:"id"`
	Title         string          `json:"title"`
	Message       string          `json:"message"`
	TargetPaneID  string          `json:"target_pane_id"`
	OrderIndex    int             `json:"order_index"`
	Status        QueueItemStatus `json:"status"`
	CreatedAt     string          `json:"created_at"`
	StartedAt     string          `json:"started_at,omitempty"`
	CompletedAt   string          `json:"completed_at,omitempty"`
	ErrorMessage  string          `json:"error_message,omitempty"`
	ResultMessage string          `json:"result_message,omitempty"`
	ClearBefore   bool            `json:"clear_before"`
	// ClearCommand stores the user-configured clear command.
	// When ClearBefore is true and this field is empty, the runtime falls back to DefaultClearCommand.
	ClearCommand string `json:"clear_command,omitempty"`
}

// QueueStatus is the frontend-safe snapshot for the queue state.
type QueueStatus struct {
	Items     []QueueItem    `json:"items"`
	RunStatus QueueRunStatus `json:"run_status"`
	// CurrentIndex is -1 when no task is actively executing.
	CurrentIndex   int    `json:"current_index"`
	SessionName    string `json:"session_name"`
	GenerationID   string `json:"generation_id"`
	ClearDelaySec  int    `json:"clear_delay_sec"`
	LastStopReason string `json:"last_stop_reason,omitempty"`
}
