package singletaskrunner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	maxTaskTitleLen       = 200
	maxTaskMessageLen     = 8000
	maxTaskResultLen      = 4000
	maxTaskFailureReason  = 2000
	defaultCancelReason   = "Cancelled"
	defaultStoppedReason  = "Stopped by user"
	defaultShutdownReason = "Application shutdown"
	completeTaskToolName  = "complete_task"
	failTaskToolName      = "fail_task"
	cancelTaskToolName    = "cancel_task"
)

var errServiceRetired = errors.New("service is retired")

// ResolutionToolNames lists the MCP tools that can resolve an active queued task.
const ResolutionToolNames = completeTaskToolName + ", " + failTaskToolName + ", or " + cancelTaskToolName

type completionSignal struct {
	finalStatus QueueItemStatus
	message     string
}

type clearPreStepResult uint8

const (
	clearPreStepOK clearPreStepResult = iota
	clearPreStepCancelled
	clearPreStepFailed
)

// EnqueueTaskInput holds raw task input for batch queue insertion; validated during EnqueueTasks.
type EnqueueTaskInput struct {
	Title        string
	Message      string
	ClearBefore  bool
	ClearCommand string
}

// EnqueuedTask contains the queue metadata returned after batch insertion.
type EnqueuedTask struct {
	TaskID     string
	OrderIndex int
}

// Service manages queue state, sequential execution, and MCP completion signals.
//
// Lock ordering: sendMu → mu → completionMu.
// Never acquire sendMu while holding mu, and never acquire mu while holding
// completionMu.
//
// Invariants:
//   - currentIndex == -1 when runStatus != QueueRunning
//   - cancel != nil when runStatus == QueueRunning
type Service struct {
	deps Deps

	mu             sync.Mutex
	sessionName    string
	items          []QueueItem
	runStatus      QueueRunStatus
	currentIndex   int
	clearDelaySec  int
	cancel         context.CancelFunc
	generationID   string
	lastStopReason string
	retired        bool
	sendMu         sync.Mutex

	// completionCh holds buffered (cap=1) channels for each active task.
	// A channel is registered when executeItem begins waiting, and removed
	// either when a signal is consumed or by the deferred removeCompletionChannel.
	// Protected by completionMu.
	completionMu sync.Mutex
	completionCh map[string]chan completionSignal
}

// NewService creates a single-task-runner service.
func NewService(deps Deps) *Service {
	deps.validateRequired()
	deps.applyDefaults()
	return &Service{
		deps:          deps,
		sessionName:   deps.SessionName,
		items:         []QueueItem{},
		runStatus:     QueueIdle,
		currentIndex:  -1,
		clearDelaySec: DefaultClearDelay,
		generationID:  uuid.NewString(),
		completionCh:  make(map[string]chan completionSignal),
	}
}

// Start begins executing all pending queue items.
func (s *Service) Start() error {
	s.mu.Lock()
	if err := s.ensureAcceptingNewWorkLocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	s.mu.Unlock()

	ctx, cancel := s.deps.NewContext()
	if err := ctx.Err(); err != nil {
		cancel()
		return fmt.Errorf("runtime context is unavailable: %w", err)
	}

	s.mu.Lock()
	if err := s.ensureAcceptingNewWorkLocked(); err != nil {
		s.mu.Unlock()
		cancel()
		return err
	}
	switch s.runStatus {
	case QueueIdle, QueueCompleted:
		// Valid: start from idle or restart after completion.
	case QueueRunning:
		s.mu.Unlock()
		cancel()
		return errors.New("queue is already running")
	default:
		s.mu.Unlock()
		cancel()
		return fmt.Errorf("cannot start queue from state %s", s.runStatus)
	}

	hasPending := false
	for i := range s.items {
		if s.items[i].Status == ItemStatusPending {
			hasPending = true
			break
		}
	}
	if !hasPending {
		s.mu.Unlock()
		cancel()
		return errors.New("no pending tasks to start")
	}
	runGenerationID := uuid.NewString()
	s.runStatus = QueueRunning
	s.currentIndex = -1
	s.cancel = cancel
	s.generationID = runGenerationID
	s.lastStopReason = ""
	s.mu.Unlock()

	s.launchWorker(ctx, runGenerationID)
	s.emitUpdated()

	slog.Debug("[DEBUG-SINGLE-TASK-RUNNER] started")
	return nil
}

// Stop stops the queue. The active task is marked as cancelled and remaining
// pending items stay pending.
func (s *Service) Stop() error {
	stopReason := defaultStoppedReason
	s.mu.Lock()
	if err := s.ensureNotRetiredLocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	if s.runStatus != QueueRunning {
		s.mu.Unlock()
		return errors.New("queue is not running")
	}
	s.cancelActiveTaskLocked(defaultStoppedReason)
	s.lastStopReason = defaultStoppedReason
	cancel := s.resetRunStateLocked()
	eventSessionName := s.sessionName
	eventGenerationID := s.generationID
	s.mu.Unlock()
	cancel()

	s.emitStopped(stopReason, eventSessionName, eventGenerationID)
	s.emitUpdated()
	slog.Debug("[DEBUG-SINGLE-TASK-RUNNER] stopped")
	return nil
}

// StopAll stops the queue during application shutdown.
func (s *Service) StopAll() {
	s.mu.Lock()
	if s.runStatus != QueueRunning {
		s.mu.Unlock()
		return
	}
	s.cancelActiveTaskLocked(defaultShutdownReason)
	s.lastStopReason = defaultShutdownReason
	cancel := s.resetRunStateLocked()
	eventSessionName := s.sessionName
	eventGenerationID := s.generationID
	s.mu.Unlock()
	cancel()

	s.emitStopped(defaultShutdownReason, eventSessionName, eventGenerationID)
	s.emitUpdated()
}

// AddItem appends a new task to the queue.
func (s *Service) AddItem(title, message, targetPaneID string, clearBefore bool, clearCommand string) error {
	item, err := s.newQueueItem(title, message, targetPaneID, clearBefore, clearCommand)
	if err != nil {
		return err
	}

	s.mu.Lock()
	if err := s.ensureAcceptingNewWorkLocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	item.OrderIndex = len(s.items)
	s.items = append(s.items, item)
	if s.runStatus != QueueRunning {
		s.runStatus = QueueIdle
	}
	s.mu.Unlock()

	s.emitUpdated()
	return nil
}

// EnqueueTasks appends a batch of tasks for the same target pane.
func (s *Service) EnqueueTasks(targetPaneID string, tasks []EnqueueTaskInput) ([]EnqueuedTask, error) {
	targetPaneID = strings.TrimSpace(targetPaneID)
	if targetPaneID == "" {
		return nil, errors.New("target pane id is required")
	}
	if len(tasks) == 0 {
		return nil, errors.New("at least one task is required")
	}
	if err := s.deps.CheckPaneAlive(targetPaneID); err != nil {
		return nil, fmt.Errorf("target pane unavailable: %w", err)
	}

	// Build items outside the lock to avoid blocking on UUID generation / time.Now.
	prebuilt := make([]QueueItem, 0, len(tasks))
	for _, task := range tasks {
		item, err := s.newQueueItemWithoutPaneCheck(task.Title, task.Message, targetPaneID, task.ClearBefore, task.ClearCommand)
		if err != nil {
			return nil, err
		}
		prebuilt = append(prebuilt, item)
	}

	s.mu.Lock()
	if err := s.ensureAcceptingNewWorkLocked(); err != nil {
		s.mu.Unlock()
		return nil, err
	}
	queued := make([]EnqueuedTask, 0, len(prebuilt))
	for _, item := range prebuilt {
		item.OrderIndex = len(s.items)
		s.items = append(s.items, item)
		queued = append(queued, EnqueuedTask{
			TaskID:     item.ID,
			OrderIndex: item.OrderIndex,
		})
	}
	if s.runStatus != QueueRunning {
		s.runStatus = QueueIdle
	}
	s.mu.Unlock()

	s.emitUpdated()
	return queued, nil
}

// RemoveItem removes a non-running item from the queue.
func (s *Service) RemoveItem(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("item id is required")
	}

	s.mu.Lock()
	if err := s.ensureNotRetiredLocked(); err != nil {
		s.mu.Unlock()
		return err
	}

	filtered := make([]QueueItem, 0, len(s.items))
	found := false
	for _, item := range s.items {
		if item.ID != id {
			filtered = append(filtered, item)
			continue
		}
		if !item.Status.IsEditable() {
			s.mu.Unlock()
			return fmt.Errorf("cannot remove item %s with status %s", id, item.Status)
		}
		found = true
	}
	if !found {
		s.mu.Unlock()
		return fmt.Errorf("item %s not found", id)
	}

	for i := range filtered {
		filtered[i].OrderIndex = i
	}
	s.items = filtered
	if len(s.items) == 0 {
		s.runStatus = QueueIdle
	}
	s.mu.Unlock()

	s.emitUpdated()
	return nil
}

// ReorderItems reorders items by their IDs.
func (s *Service) ReorderItems(orderedIDs []string) error {
	if len(orderedIDs) == 0 {
		return errors.New("ordered ids are required")
	}

	seen := make(map[string]struct{}, len(orderedIDs))
	for _, id := range orderedIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			return errors.New("ordered ids must not contain empty values")
		}
		if _, ok := seen[trimmed]; ok {
			return fmt.Errorf("duplicate id %s in ordered ids", trimmed)
		}
		seen[trimmed] = struct{}{}
	}

	s.mu.Lock()
	if err := s.ensureNotRetiredLocked(); err != nil {
		s.mu.Unlock()
		return err
	}

	// QueueIdle and QueueCompleted are both reorderable snapshots.
	if s.runStatus == QueueRunning {
		s.mu.Unlock()
		return errors.New("cannot reorder while queue is running")
	}

	itemMap := make(map[string]QueueItem, len(s.items))
	for _, item := range s.items {
		itemMap[item.ID] = item
	}
	if len(itemMap) != len(orderedIDs) {
		s.mu.Unlock()
		return fmt.Errorf("ordered ids count %d does not match items count %d", len(orderedIDs), len(itemMap))
	}

	reordered := make([]QueueItem, 0, len(orderedIDs))
	for i, id := range orderedIDs {
		item, ok := itemMap[strings.TrimSpace(id)]
		if !ok {
			s.mu.Unlock()
			return fmt.Errorf("item %s not found", id)
		}
		item.OrderIndex = i
		reordered = append(reordered, item)
	}
	s.items = reordered
	s.mu.Unlock()

	s.emitUpdated()
	return nil
}

// UpdateItem modifies an editable item (pending, done, failed, or cancelled).
// Items in a terminal state (done/failed/cancelled) are reset to pending before applying changes.
func (s *Service) UpdateItem(id, title, message, targetPaneID string, clearBefore bool, clearCommand string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("item id is required")
	}

	title, err := validateTaskTitle(title)
	if err != nil {
		return err
	}
	message, err = validateTaskMessage(message)
	if err != nil {
		return err
	}
	targetPaneID = strings.TrimSpace(targetPaneID)
	if targetPaneID == "" {
		return errors.New("target pane id is required")
	}
	if err := s.deps.CheckPaneAlive(targetPaneID); err != nil {
		return fmt.Errorf("target pane unavailable: %w", err)
	}
	clearCommand = strings.TrimSpace(clearCommand)

	s.mu.Lock()
	if err := s.ensureNotRetiredLocked(); err != nil {
		s.mu.Unlock()
		return err
	}

	for i := range s.items {
		if s.items[i].ID != id {
			continue
		}
		if !s.items[i].Status.IsEditable() {
			s.mu.Unlock()
			return fmt.Errorf("cannot update item %s with status %s", id, s.items[i].Status)
		}
		if s.items[i].Status != ItemStatusPending {
			s.items[i].Status = ItemStatusPending
			s.items[i].StartedAt = ""
			s.items[i].CompletedAt = ""
			s.items[i].ErrorMessage = ""
			s.items[i].ResultMessage = ""
			if s.runStatus != QueueRunning {
				s.runStatus = QueueIdle
			}
		}
		s.items[i].Title = title
		s.items[i].Message = message
		s.items[i].TargetPaneID = targetPaneID
		s.items[i].ClearBefore = clearBefore
		s.items[i].ClearCommand = clearCommand
		s.mu.Unlock()
		s.emitUpdated()
		return nil
	}

	s.mu.Unlock()
	return fmt.Errorf("item %s not found", id)
}

// SetClearDelay updates the clear-command wait time in seconds.
func (s *Service) SetClearDelay(delaySec int) error {
	if delaySec < MinClearDelaySec || delaySec > MaxClearDelaySec {
		return fmt.Errorf("clear delay must be between %d and %d seconds", MinClearDelaySec, MaxClearDelaySec)
	}

	s.mu.Lock()
	if err := s.ensureNotRetiredLocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	s.clearDelaySec = delaySec
	s.mu.Unlock()

	s.emitUpdated()
	return nil
}

// GetClearDelay returns the current clear-command wait time.
func (s *Service) GetClearDelay() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.retired {
		return DefaultClearDelay
	}
	return s.clearDelaySec
}

// GetStatus returns a copy of the current queue status.
func (s *Service) GetStatus() QueueStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.retired {
		return QueueStatus{
			Items:          []QueueItem{},
			RunStatus:      QueueIdle,
			CurrentIndex:   -1,
			SessionName:    "",
			GenerationID:   "",
			ClearDelaySec:  DefaultClearDelay,
			LastStopReason: "",
		}
	}

	items := make([]QueueItem, len(s.items))
	copy(items, s.items)

	return QueueStatus{
		Items:          items,
		RunStatus:      s.runStatus,
		CurrentIndex:   s.currentIndex,
		SessionName:    s.sessionName,
		GenerationID:   s.generationID,
		ClearDelaySec:  s.clearDelaySec,
		LastStopReason: s.lastStopReason,
	}
}

// RenameSession rebinds the service to a renamed session without discarding
// the existing queue state.
func (s *Service) RenameSession(newSessionName string) error {
	newSessionName = strings.TrimSpace(newSessionName)
	if newSessionName == "" {
		return errors.New("new session name is required")
	}

	s.mu.Lock()
	if err := s.ensureNotRetiredLocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	s.sessionName = newSessionName
	s.mu.Unlock()
	return nil
}

// Retire suppresses future frontend-facing events from a service that has been
// detached from the manager. External callers are rejected or downgraded to
// default read-only snapshots after retirement.
func (s *Service) Retire() {
	s.mu.Lock()
	s.retired = true
	s.mu.Unlock()
}

func (s *Service) shouldEmit() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.retired && !s.deps.IsShuttingDown()
}

func (s *Service) ensureNotRetiredLocked() error {
	if !s.retired {
		return nil
	}
	return errServiceRetired
}

func (s *Service) ensureAcceptingNewWorkLocked() error {
	// Keep new-work admission behind a named gate so future preconditions can be
	// added without rewriting every caller.
	return s.ensureNotRetiredLocked()
}

// CompleteTask notifies the queue that the active task finished successfully.
func (s *Service) CompleteTask(taskID, result string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return errors.New("task id is required")
	}
	result = strings.TrimSpace(result)
	if len([]rune(result)) > maxTaskResultLen {
		return fmt.Errorf("result must be %d characters or fewer", maxTaskResultLen)
	}

	s.mu.Lock()
	if err := s.ensureNotRetiredLocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	s.mu.Unlock()

	if err := s.ensureCompletableStatus(taskID); err != nil {
		return err
	}
	if !s.signalCompletion(taskID, completionSignal{finalStatus: ItemStatusDone, message: result}) {
		return s.completionAwaitError(taskID)
	}
	return nil
}

// FailTask notifies the queue that the active task failed.
func (s *Service) FailTask(taskID, reason string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return errors.New("task id is required")
	}
	reason = strings.TrimSpace(reason)
	if len([]rune(reason)) > maxTaskFailureReason {
		return fmt.Errorf("reason must be %d characters or fewer", maxTaskFailureReason)
	}

	s.mu.Lock()
	if err := s.ensureNotRetiredLocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	s.mu.Unlock()

	if err := s.ensureCompletableStatus(taskID); err != nil {
		return err
	}
	if !s.signalCompletion(taskID, completionSignal{finalStatus: ItemStatusFailed, message: reason}) {
		return s.completionAwaitError(taskID)
	}
	return nil
}

// CancelTask cancels a pending or active task.
func (s *Service) CancelTask(taskID, reason string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return errors.New("task id is required")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = defaultCancelReason
	}
	if len([]rune(reason)) > maxTaskFailureReason {
		return fmt.Errorf("reason must be %d characters or fewer", maxTaskFailureReason)
	}

	s.mu.Lock()
	if err := s.ensureNotRetiredLocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	idx := s.findItemIndexByIDLocked(taskID)
	if idx == -1 {
		s.mu.Unlock()
		return fmt.Errorf("task %s not found", taskID)
	}

	status := s.items[idx].Status
	now := time.Now().UTC().Format(time.RFC3339)
	switch status {
	case ItemStatusPending:
		s.items[idx].Status = ItemStatusCancelled
		s.items[idx].CompletedAt = now
		s.items[idx].ErrorMessage = reason
		s.items[idx].ResultMessage = ""
		s.mu.Unlock()
		s.emitUpdated()
		return nil
	case ItemStatusSending, ItemStatusActive:
		if status == ItemStatusSending {
			s.mu.Unlock()
			s.sendMu.Lock()
			s.mu.Lock()
			idx = s.findItemIndexByIDLocked(taskID)
			if idx == -1 {
				s.mu.Unlock()
				s.sendMu.Unlock()
				return fmt.Errorf("task %s not found", taskID)
			}
			status = s.items[idx].Status
			if status != ItemStatusSending && status != ItemStatusActive {
				s.mu.Unlock()
				s.sendMu.Unlock()
				return fmt.Errorf("cannot cancel task %s with status %s", taskID, status)
			}
		}
		s.items[idx].Status = ItemStatusCancelled
		s.items[idx].CompletedAt = now
		s.items[idx].ErrorMessage = reason
		s.items[idx].ResultMessage = ""
		// Lock order: mu (held) → completionMu (matches cancelActiveTaskLocked)
		s.completionMu.Lock()
		signaled := s.signalCompletionLocked(taskID, completionSignal{finalStatus: ItemStatusCancelled, message: reason})
		s.completionMu.Unlock()
		s.mu.Unlock()
		if status == ItemStatusSending {
			s.sendMu.Unlock()
		}
		if !signaled {
			if status == ItemStatusActive {
				// Active tasks should always have a completion channel.
				slog.Warn("[SINGLE-TASK-RUNNER] CancelTask: active task has no completion channel",
					"taskID", taskID)
			} else {
				// Sending tasks may not have a channel yet; this is normal.
				slog.Debug("[DEBUG-SINGLE-TASK-RUNNER] CancelTask: no completion channel (sending phase)",
					"taskID", taskID)
			}
		}
		s.emitUpdated()
		return nil
	default:
		s.mu.Unlock()
		return fmt.Errorf("cannot cancel task %s with status %s", taskID, status)
	}
}

func (s *Service) launchWorker(ctx context.Context, generationID string) {
	recoveryOpts := s.deps.BaseRecoveryOptions()
	recoveryOpts.MaxRetries = 1
	origOnFatal := recoveryOpts.OnFatal
	recoveryOpts.OnFatal = func(worker string, maxRetries int) {
		s.handleWorkerFatal(generationID, "internal panic")
		if origOnFatal != nil {
			origOnFatal(worker, maxRetries)
		}
	}

	s.deps.LaunchWorker("single-task-runner", ctx, func(ctx context.Context) {
		s.runLoop(ctx, generationID)
	}, recoveryOpts)
}

func (s *Service) runLoop(ctx context.Context, generationID string) {
	defer func() {
		s.mu.Lock()
		if s.runStatus != QueueRunning || !s.isCurrentGenerationLocked(generationID) {
			s.mu.Unlock()
			return
		}
		s.cancelActiveTaskLocked("runLoop terminated")
		s.lastStopReason = "runLoop terminated"
		cancel := s.resetRunStateLocked()
		s.mu.Unlock()
		cancel()
		s.emitUpdated()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		s.mu.Lock()
		if s.runStatus != QueueRunning || !s.isCurrentGenerationLocked(generationID) {
			s.mu.Unlock()
			return
		}

		nextIdx := -1
		for i := range s.items {
			if s.items[i].Status == ItemStatusPending {
				nextIdx = i
				break
			}
		}
		if nextIdx == -1 {
			cancel := s.setRunStateLocked(QueueCompleted)
			s.lastStopReason = ""
			s.mu.Unlock()
			cancel()
			s.emitUpdated()
			slog.Debug("[DEBUG-SINGLE-TASK-RUNNER] queue completed")
			return
		}

		s.currentIndex = nextIdx
		item := s.items[nextIdx]
		s.mu.Unlock()

		s.emitUpdated()
		if !s.executeItem(ctx, generationID, nextIdx, item) {
			return
		}
	}
}

func (s *Service) executeItem(ctx context.Context, generationID string, idx int, item QueueItem) bool {
	now := time.Now().UTC().Format(time.RFC3339)
	s.mu.Lock()
	if !s.isCurrentGenerationLocked(generationID) {
		s.mu.Unlock()
		return false
	}
	s.items[idx].Status = ItemStatusSending
	s.items[idx].StartedAt = now
	s.items[idx].CompletedAt = ""
	s.items[idx].ErrorMessage = ""
	s.items[idx].ResultMessage = ""
	s.mu.Unlock()
	s.emitUpdated()

	ch := s.registerCompletionChannel(item.ID)
	// Ensure the completion channel is always removed when executeItem
	// returns, regardless of exit path. removeCompletionChannel is safe
	// to call even if signalCompletionLocked already deleted the entry.
	defer s.removeCompletionChannel(item.ID)
	if !s.ensureItemIsSending(generationID, idx, item.ID, "before clear step") {
		return true
	}

	if err := s.deps.CheckPaneAlive(item.TargetPaneID); err != nil {
		s.failItem(generationID, idx, item.ID, fmt.Sprintf("target pane unavailable: %v", err))
		return false
	}
	switch s.executeClearPreStep(ctx, generationID, idx, item) {
	case clearPreStepOK:
	case clearPreStepCancelled:
		return true
	default:
		return false
	}

	fullMessage := buildTaskMessage(item.ID, item.Message)
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	if !s.ensureItemIsSending(generationID, idx, item.ID, "before paste send") {
		return true
	}
	if err := s.deps.SendMessagePaste(item.TargetPaneID, fullMessage); err != nil {
		s.failItem(generationID, idx, item.ID, fmt.Sprintf("send message: %v", err))
		return false
	}

	s.mu.Lock()
	if idx >= 0 && idx < len(s.items) && s.items[idx].ID == item.ID && s.isCurrentGenerationLocked(generationID) {
		if s.items[idx].Status != ItemStatusSending {
			// Status was changed externally (e.g., CancelTask). Abort execution.
			slog.Debug("[DEBUG-SINGLE-TASK-RUNNER] task status changed during send, skipping",
				"taskID", item.ID, "currentStatus", s.items[idx].Status)
			s.mu.Unlock()
			return true
		}
		s.items[idx].Status = ItemStatusActive
		s.mu.Unlock()
		s.emitUpdated()
		slog.Debug("[DEBUG-SINGLE-TASK-RUNNER] task sent", "taskID", item.ID, "pane", item.TargetPaneID)
	} else {
		s.mu.Unlock()
		s.handleWorkerFatal(generationID, "task queue mutated during execution")
		return false
	}

	var signal completionSignal
	select {
	case <-ctx.Done():
		return false
	case signal = <-ch:
	}
	if !s.isCurrentGeneration(generationID) {
		return false
	}

	switch signal.finalStatus {
	case ItemStatusDone:
		completedAt := time.Now().UTC().Format(time.RFC3339)
		s.mu.Lock()
		if idx >= 0 && idx < len(s.items) && s.items[idx].ID == item.ID && s.isCurrentGenerationLocked(generationID) {
			// If CancelTask already set the status to Cancelled, honour
			// the cancellation rather than overwriting with Done.
			if s.items[idx].Status == ItemStatusCancelled {
				s.mu.Unlock()
				s.emitUpdated()
				return true
			}
			s.items[idx].Status = ItemStatusDone
			s.items[idx].CompletedAt = completedAt
			s.items[idx].ErrorMessage = ""
			s.items[idx].ResultMessage = signal.message
		}
		s.mu.Unlock()
		s.emitUpdated()
		return true
	case ItemStatusCancelled:
		s.mu.Lock()
		if idx >= 0 && idx < len(s.items) && s.items[idx].ID == item.ID && s.isCurrentGenerationLocked(generationID) {
			s.items[idx].Status = ItemStatusCancelled
			if s.items[idx].CompletedAt == "" {
				s.items[idx].CompletedAt = time.Now().UTC().Format(time.RFC3339)
			}
			s.items[idx].ErrorMessage = signal.message
			s.items[idx].ResultMessage = ""
		}
		s.mu.Unlock()
		s.emitUpdated()
		return true
	case ItemStatusFailed:
		s.failItem(generationID, idx, item.ID, signal.message)
		return false
	default:
		s.failItem(generationID, idx, item.ID, fmt.Sprintf("task finished with unknown status: %s", signal.finalStatus))
		return false
	}
}

func (s *Service) executeClearPreStep(ctx context.Context, generationID string, idx int, item QueueItem) clearPreStepResult {
	if !item.ClearBefore {
		return clearPreStepOK
	}

	clearCmd := item.ClearCommand
	if clearCmd == "" {
		clearCmd = DefaultClearCommand
	}

	if err := s.deps.SendClearCommand(item.TargetPaneID, clearCmd); err != nil {
		s.failItem(generationID, idx, item.ID, fmt.Sprintf("clear command: %v", err))
		return clearPreStepFailed
	}

	delaySec := s.GetClearDelay()
	if delaySec <= 0 {
		return clearPreStepOK
	}

	timer := time.NewTimer(time.Duration(delaySec) * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return clearPreStepFailed
		case <-timer.C:
			return clearPreStepOK
		case <-ticker.C:
			status, ok := s.itemStatusSnapshot(generationID, idx, item.ID)
			if !ok {
				return clearPreStepFailed
			}
			if status != ItemStatusSending {
				return clearPreStepCancelled
			}
		}
	}
}

func (s *Service) failItem(generationID string, idx int, itemID, reason string) {
	now := time.Now().UTC().Format(time.RFC3339)
	eventSessionName := ""
	eventGenerationID := ""

	s.mu.Lock()
	if !s.isCurrentGenerationLocked(generationID) {
		currentGenerationID := s.generationID
		currentRunStatus := s.runStatus
		s.mu.Unlock()
		slog.Warn("[WARN-SINGLE-TASK-RUNNER] ignored stale generation failure",
			"idx", idx,
			"itemID", itemID,
			"staleGenerationID", generationID,
			"currentGenerationID", currentGenerationID,
			"currentRunStatus", currentRunStatus,
			"reason", reason)
		return
	}
	if idx >= 0 && idx < len(s.items) && s.items[idx].ID == itemID {
		s.items[idx].Status = ItemStatusFailed
		s.items[idx].CompletedAt = now
		s.items[idx].ErrorMessage = reason
		s.items[idx].ResultMessage = ""
	}
	s.lastStopReason = reason
	cancel := s.resetRunStateLocked()
	eventSessionName = s.sessionName
	eventGenerationID = s.generationID
	s.mu.Unlock()

	cancel()
	slog.Warn("[SINGLE-TASK-RUNNER] item failed", "index", idx, "reason", reason)
	s.emitStopped(reason, eventSessionName, eventGenerationID)
	s.emitUpdated()
}

func (s *Service) handleWorkerFatal(generationID string, reason string) {
	slog.Error("[SINGLE-TASK-RUNNER] worker fatal", "reason", reason, "session", s.sessionNameSnapshot())

	eventSessionName := ""
	eventGenerationID := ""
	s.mu.Lock()
	if !s.isCurrentGenerationLocked(generationID) {
		currentGenerationID := s.generationID
		currentRunStatus := s.runStatus
		s.mu.Unlock()
		slog.Warn("[WARN-SINGLE-TASK-RUNNER] ignored stale worker fatal",
			"staleGenerationID", generationID,
			"currentGenerationID", currentGenerationID,
			"currentRunStatus", currentRunStatus,
			"reason", reason)
		return
	}
	s.cancelActiveTaskLocked(reason)
	s.clearAllCompletionChannelsLocked()
	s.lastStopReason = reason
	cancel := s.resetRunStateLocked()
	eventSessionName = s.sessionName
	eventGenerationID = s.generationID
	s.mu.Unlock()

	cancel()
	s.emitStopped(reason, eventSessionName, eventGenerationID)
	s.emitUpdated()
}

func (s *Service) ensureCompletableStatus(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureNotRetiredLocked(); err != nil {
		return err
	}

	idx := s.findItemIndexByIDLocked(taskID)
	if idx == -1 {
		return fmt.Errorf("task %s not found", taskID)
	}
	switch s.items[idx].Status {
	case ItemStatusSending, ItemStatusActive:
		return nil
	default:
		return fmt.Errorf("task %s is not active", taskID)
	}
}

func (s *Service) completionAwaitError(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureNotRetiredLocked(); err != nil {
		return err
	}

	idx := s.findItemIndexByIDLocked(taskID)
	if idx == -1 {
		return fmt.Errorf("task %s is not awaiting completion (it may have already finished or been cancelled)", taskID)
	}

	status := s.items[idx].Status
	if status == ItemStatusSending || status == ItemStatusActive {
		return fmt.Errorf("task %s is not awaiting completion (status %s lost its completion channel; it may have been concurrently cancelled)", taskID, status)
	}
	return fmt.Errorf("task %s is not awaiting completion (current status: %s)", taskID, status)
}

// resetRunStateLocked resets the queue to idle and returns the cancel function.
// The returned function is always safe to call (nil-guarded internally).
// Must be called with s.mu held.
func (s *Service) resetRunStateLocked() context.CancelFunc {
	return s.setRunStateLocked(QueueIdle)
}

// setRunStateLocked updates the queue run state and detaches the current cancel
// function. The returned function is always safe to call and must be invoked
// after releasing s.mu.
func (s *Service) setRunStateLocked(status QueueRunStatus) context.CancelFunc {
	s.runStatus = status
	s.currentIndex = -1
	cancel := s.cancel
	s.cancel = nil
	if cancel == nil {
		return func() {}
	}
	return cancel
}

func (s *Service) ensureItemIsSending(generationID string, idx int, itemID, phase string) bool {
	status, ok := s.itemStatusSnapshot(generationID, idx, itemID)
	if !ok {
		s.handleWorkerFatal(generationID, "task queue mutated during execution")
		return false
	}
	if status == ItemStatusSending {
		return true
	}
	slog.Debug("[DEBUG-SINGLE-TASK-RUNNER] task status changed during send, skipping step",
		"taskID", itemID,
		"phase", phase,
		"currentStatus", status)
	return false
}

func (s *Service) findItemIndexByIDLocked(taskID string) int {
	for i := range s.items {
		if s.items[i].ID == taskID {
			return i
		}
	}
	return -1
}

func (s *Service) itemStatusSnapshot(generationID string, idx int, itemID string) (QueueItemStatus, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isCurrentGenerationLocked(generationID) || idx < 0 || idx >= len(s.items) || s.items[idx].ID != itemID {
		return "", false
	}
	return s.items[idx].Status, true
}

func (s *Service) isCurrentGeneration(generationID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isCurrentGenerationLocked(generationID)
}

func (s *Service) isCurrentGenerationLocked(generationID string) bool {
	return s.generationID == generationID
}

func (s *Service) cancelActiveTaskLocked(reason string) {
	now := time.Now().UTC().Format(time.RFC3339)
	for i := range s.items {
		switch s.items[i].Status {
		case ItemStatusSending, ItemStatusActive:
			s.items[i].Status = ItemStatusCancelled
			s.items[i].CompletedAt = now
			s.items[i].ErrorMessage = reason
			s.items[i].ResultMessage = ""
			// Lock order: mu (already held by caller) → completionMu
			s.completionMu.Lock()
			s.signalCompletionLocked(s.items[i].ID, completionSignal{
				finalStatus: ItemStatusCancelled,
				message:     reason,
			})
			s.completionMu.Unlock()
		}
	}
}

func (s *Service) registerCompletionChannel(taskID string) chan completionSignal {
	ch := make(chan completionSignal, 1)
	s.completionMu.Lock()
	s.completionCh[taskID] = ch
	s.completionMu.Unlock()
	return ch
}

func (s *Service) removeCompletionChannel(taskID string) {
	s.completionMu.Lock()
	delete(s.completionCh, taskID)
	s.completionMu.Unlock()
}

func (s *Service) clearAllCompletionChannelsLocked() {
	s.completionMu.Lock()
	defer s.completionMu.Unlock()
	clear(s.completionCh)
}

func (s *Service) signalCompletion(taskID string, signal completionSignal) bool {
	s.completionMu.Lock()
	defer s.completionMu.Unlock()
	return s.signalCompletionLocked(taskID, signal)
}

func (s *Service) signalCompletionLocked(taskID string, signal completionSignal) bool {
	ch, ok := s.completionCh[taskID]
	if !ok {
		slog.Debug("[DEBUG-SINGLE-TASK-RUNNER] signalCompletion: no channel registered",
			"taskID", taskID, "signal", signal.finalStatus)
		return false
	}

	select {
	case ch <- signal:
		delete(s.completionCh, taskID)
		return true
	default:
		// A duplicate signal indicates a logic issue (e.g. concurrent
		// Complete + Cancel). Log at Warn so it is visible in production.
		select {
		case existing := <-ch:
			slog.Warn("[SINGLE-TASK-RUNNER] signalCompletion: duplicate signal dropped",
				"taskID", taskID,
				"kept", existing.finalStatus,
				"dropped", signal.finalStatus)
			ch <- existing
		default:
			slog.Warn("[SINGLE-TASK-RUNNER] signalCompletion: duplicate signal dropped",
				"taskID", taskID,
				"dropped", signal.finalStatus)
		}
		delete(s.completionCh, taskID)
		return false
	}
}

func (s *Service) newQueueItem(title, message, targetPaneID string, clearBefore bool, clearCommand string) (QueueItem, error) {
	title, message, targetPaneID, clearCommand, err := normalizeQueueItemInput(title, message, targetPaneID, clearCommand)
	if err != nil {
		return QueueItem{}, err
	}
	if err := s.deps.CheckPaneAlive(targetPaneID); err != nil {
		return QueueItem{}, fmt.Errorf("target pane unavailable: %w", err)
	}
	return buildQueueItem(title, message, targetPaneID, clearBefore, clearCommand), nil
}

func (s *Service) newQueueItemWithoutPaneCheck(title, message, targetPaneID string, clearBefore bool, clearCommand string) (QueueItem, error) {
	title, message, targetPaneID, clearCommand, err := normalizeQueueItemInput(title, message, targetPaneID, clearCommand)
	if err != nil {
		return QueueItem{}, err
	}
	return buildQueueItem(title, message, targetPaneID, clearBefore, clearCommand), nil
}

func normalizeQueueItemInput(title, message, targetPaneID, clearCommand string) (string, string, string, string, error) {
	title, err := validateTaskTitle(title)
	if err != nil {
		return "", "", "", "", err
	}
	message, err = validateTaskMessage(message)
	if err != nil {
		return "", "", "", "", err
	}
	targetPaneID = strings.TrimSpace(targetPaneID)
	if targetPaneID == "" {
		return "", "", "", "", errors.New("target pane id is required")
	}
	clearCommand = strings.TrimSpace(clearCommand)
	return title, message, targetPaneID, clearCommand, nil
}

// buildQueueItem creates a QueueItem with all fields except OrderIndex, which
// is set by the caller under mu.Lock once the insertion position is known.
func buildQueueItem(title, message, targetPaneID string, clearBefore bool, clearCommand string) QueueItem {
	now := time.Now().UTC().Format(time.RFC3339)
	return QueueItem{
		ID:           "t-" + uuid.NewString(),
		Title:        title,
		Message:      message,
		TargetPaneID: targetPaneID,
		Status:       ItemStatusPending,
		CreatedAt:    now,
		ClearBefore:  clearBefore,
		ClearCommand: clearCommand,
	}
}

func validateTaskTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", errors.New("title is required")
	}
	if len([]rune(title)) > maxTaskTitleLen {
		return "", fmt.Errorf("title must be %d characters or fewer", maxTaskTitleLen)
	}
	return title, nil
}

func validateTaskMessage(message string) (string, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return "", errors.New("message is required")
	}
	if len([]rune(message)) > maxTaskMessageLen {
		return "", fmt.Errorf("message must be %d characters or fewer", maxTaskMessageLen)
	}
	return message, nil
}

func (s *Service) emitUpdated() {
	if !s.shouldEmit() {
		return
	}
	s.deps.Emitter.Emit("single-task-runner:updated", s.GetStatus())
}

func (s *Service) emitStopped(reason string, sessionName string, generationID string) {
	if !s.shouldEmit() {
		return
	}
	s.deps.Emitter.Emit("single-task-runner:stopped", map[string]string{
		"reason":        reason,
		"session_name":  sessionName,
		"generation_id": generationID,
	})
}

func (s *Service) sessionNameSnapshot() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessionName
}

func buildTaskMessage(taskID, message string) string {
	trimmedMessage := strings.TrimSpace(message)
	if taskID == "" {
		taskID = "<task_id>"
	}
	return trimmedMessage + "\n\n---\n[Task Info]\n" +
		"task_id: " + taskID + "\n" +
		"On success: call " + completeTaskToolName + " with this task_id.\n" +
		"On failure: call " + failTaskToolName + " with this task_id.\n" +
		"If the task should not continue: call " + cancelTaskToolName + " with this task_id."
}
