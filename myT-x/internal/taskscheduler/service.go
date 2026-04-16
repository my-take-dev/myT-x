package taskscheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	orcdomain "myT-x/internal/mcp/agent-orchestrator/domain"
)

const (
	// pollInterval is how often the worker checks for task completion.
	pollInterval = 10 * time.Second

	// staleTaskIDWaitInterval gives a resumed worker a short window to observe an
	// in-flight SendMessagePaste completion from the previous generation without
	// hammering the queue state in a tight loop.
	staleTaskIDWaitInterval = 100 * time.Millisecond

	// staleTaskIDWaitTimeout bounds how long a resumed worker waits for an
	// in-flight SendMessagePaste call to publish the first task ID.
	staleTaskIDWaitTimeout = 30 * time.Second

	// maxConsecutivePollErrors bounds completion polling when the orchestrator DB
	// stays unavailable. This prevents the queue from remaining stuck in running
	// forever on permanent database failures.
	maxConsecutivePollErrors = 3

	// defaultClearCommand is the default command sent to clear context before a task.
	defaultClearCommand = "/new"

	// clearCommandDelay is the wait time after sending a clear command.
	// 2 seconds is sufficient for typical AI tool commands like /new to be
	// accepted and processed before the next task message is sent.
	clearCommandDelay = 2 * time.Second

	defaultStoppedReason  = "Stopped by user"
	defaultShutdownReason = "Application shutdown"
)

var errServiceRetired = errors.New("service is retired")

// Service manages the task scheduler queue, execution, and completion detection.
//
// Thread-safety is managed internally via mu (queue state) and dbMu
// (orchestrator DB access). No external locking is required.
//
// Lock ordering: dbMu -> mu when both are required.
type Service struct {
	deps Deps
	mu   sync.Mutex

	// Queue state (protected by mu).
	sessionName       string
	generationID      string
	items             []QueueItem
	config            QueueConfig
	runStatus         QueueRunStatus
	currentIndex      int               // -1 when not running
	preExecProgress   string            // Empty when not in the pre-execution phase.
	retired           bool              // Set by Retire(); suppresses events and rejects new work.
	pendingOrcTaskIDs map[string]string // itemID -> taskID while SendMessagePaste is still in flight.

	// Worker lifecycle (protected by mu for reads; cancel called outside lock).
	cancel context.CancelFunc

	// orchestrator DB connection (protected by dbMu, opened/closed per queue run).
	dbMu  sync.Mutex
	orcDB *orchestratorDB
}

// NewService creates a task scheduler service with the given dependencies.
// Panics if any required function field in deps is nil.
func NewService(deps Deps) *Service {
	deps.validateRequired()
	deps.applyDefaults()
	return &Service{
		deps:              deps,
		sessionName:       deps.SessionName,
		generationID:      uuid.NewString(),
		items:             []QueueItem{},
		pendingOrcTaskIDs: make(map[string]string),
		runStatus:         QueueIdle,
		currentIndex:      -1,
	}
}

// ------------------------------------------------------------
// Queue lifecycle
// ------------------------------------------------------------

// Start begins executing the task queue with the given config and items.
func (s *Service) Start(config QueueConfig, items []QueueItem) error {
	s.mu.Lock()
	if err := s.ensureNotRetiredLocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	s.mu.Unlock()

	if len(items) == 0 {
		return errors.New("at least one task item is required")
	}
	applyConfigDefaults(&config)
	if config.PreExecTargetMode != PreExecTargetModeAllPanes && config.PreExecTargetMode != PreExecTargetModeTaskPanes {
		return fmt.Errorf("invalid pre-exec target mode: %q", config.PreExecTargetMode)
	}

	// Validate all target panes exist before starting.
	for i := range items {
		items[i].Title = strings.TrimSpace(items[i].Title)
		items[i].Message = strings.TrimSpace(items[i].Message)
		items[i].TargetPaneID = strings.TrimSpace(items[i].TargetPaneID)
		items[i].ClearCommand = strings.TrimSpace(items[i].ClearCommand)
		if items[i].Title == "" {
			return fmt.Errorf("item %d: title is required", i)
		}
		if items[i].Message == "" {
			return fmt.Errorf("item %d: message is required", i)
		}
		if items[i].TargetPaneID == "" {
			return fmt.Errorf("item %d: target pane id is required", i)
		}
		if err := s.deps.CheckPaneAlive(items[i].TargetPaneID); err != nil {
			return fmt.Errorf("item %d: pane %s not available: %w", i, items[i].TargetPaneID, err)
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	prepared := make([]QueueItem, len(items))
	for i, item := range items {
		prepared[i] = QueueItem{
			ID:           uuid.New().String(),
			Title:        item.Title,
			Message:      item.Message,
			TargetPaneID: item.TargetPaneID,
			OrderIndex:   i,
			Status:       ItemStatusPending,
			CreatedAt:    now,
			ClearBefore:  item.ClearBefore,
			ClearCommand: item.ClearCommand,
		}
	}

	s.mu.Lock()
	if err := s.ensureNotRetiredLocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	if s.runStatus == QueueRunning || s.runStatus == QueuePaused || s.runStatus == QueuePreparing {
		s.mu.Unlock()
		return errors.New("queue is already running; stop it first")
	}
	runGenerationID := uuid.NewString()
	s.items = prepared
	s.config = config
	s.preExecProgress = ""
	s.generationID = runGenerationID
	if config.PreExecEnabled {
		s.runStatus = QueuePreparing
		s.currentIndex = -1
	} else {
		s.runStatus = QueueRunning
		s.currentIndex = 0
	}
	s.mu.Unlock()

	ctx, cancel := s.deps.NewContext()

	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()

	s.launchWorker(ctx, runGenerationID)
	s.emitUpdated()

	slog.Info("[DEBUG-TASK-SCHEDULER] started", "itemCount", len(prepared))
	return nil
}

// Stop stops the queue and marks remaining items as skipped.
func (s *Service) Stop() error {
	stopReason := defaultStoppedReason
	s.mu.Lock()
	if err := s.ensureNotRetiredLocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	if s.runStatus != QueueRunning && s.runStatus != QueuePaused && s.runStatus != QueuePreparing {
		s.mu.Unlock()
		return errors.New("queue is not running")
	}
	cancel := s.cancel
	s.cancel = nil
	taskIDs := s.collectTrackedOrchestratorTaskIDsLocked()
	eventSessionName := s.sessionName
	eventGenerationID := s.generationID
	for i := range s.items {
		if !IsTerminal(s.items[i].Status) {
			s.items[i].Status = ItemStatusSkipped
		}
	}
	s.pendingOrcTaskIDs = make(map[string]string)
	s.runStatus = QueueIdle
	s.currentIndex = -1
	s.preExecProgress = ""
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	cleanupErr := s.abandonTrackedTasks(taskIDs)
	s.closeOrcDB()
	s.emitStopped(stopReason, eventSessionName, eventGenerationID)
	s.emitUpdated()

	slog.Info("[DEBUG-TASK-SCHEDULER] stopped")
	if cleanupErr != nil {
		return fmt.Errorf("%s: %w", stopReason, cleanupErr)
	}
	return nil
}

// Pause pauses an actively running queue by cancelling the current worker
// context. The preparing phase is intentionally not pausable because it does
// not have resumable intermediate state.
// If an item is currently running, its status is preserved as "running".
// Resume() detects the running item and re-polls for its completion.
func (s *Service) Pause() error {
	s.mu.Lock()
	if err := s.ensureNotRetiredLocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	if s.runStatus != QueueRunning {
		s.mu.Unlock()
		return errors.New("queue is not in a pausable state")
	}
	s.runStatus = QueuePaused
	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()

	// Cancel the worker context so the goroutine exits.
	// Resume() will launch a new worker that picks up any running item.
	if cancel != nil {
		cancel()
	}
	s.closeOrcDB()
	s.emitUpdated()

	slog.Info("[DEBUG-TASK-SCHEDULER] paused")
	return nil
}

// Resume resumes a paused queue.
func (s *Service) Resume() error {
	s.mu.Lock()
	if err := s.ensureNotRetiredLocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	if s.runStatus != QueuePaused {
		s.mu.Unlock()
		return errors.New("queue is not paused")
	}
	runGenerationID := uuid.NewString()
	s.generationID = runGenerationID
	s.runStatus = QueueRunning
	s.mu.Unlock()

	ctx, cancel := s.deps.NewContext()

	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()

	s.launchWorker(ctx, runGenerationID)
	s.emitUpdated()

	slog.Info("[DEBUG-TASK-SCHEDULER] resumed")
	return nil
}

// StopAll is called during shutdown to stop the queue gracefully.
func (s *Service) StopAll() {
	s.mu.Lock()
	if s.runStatus != QueueRunning && s.runStatus != QueuePaused && s.runStatus != QueuePreparing {
		s.mu.Unlock()
		return
	}
	cancel := s.cancel
	s.cancel = nil
	taskIDs := s.collectTrackedOrchestratorTaskIDsLocked()
	eventSessionName := s.sessionName
	eventGenerationID := s.generationID
	s.pendingOrcTaskIDs = make(map[string]string)
	s.runStatus = QueueIdle
	s.currentIndex = -1
	s.preExecProgress = ""
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if err := s.abandonTrackedTasks(taskIDs); err != nil {
		slog.Warn("[WARN-TASK-SCHEDULER] stop all cleanup failed", "error", err)
	}
	s.closeOrcDB()
	s.emitStopped(defaultShutdownReason, eventSessionName, eventGenerationID)
}

// ------------------------------------------------------------
// Queue item management
// ------------------------------------------------------------

// AddItem adds a new task to the end of the queue.
func (s *Service) AddItem(title, message, targetPaneID string, clearBefore bool, clearCommand string) error {
	title = strings.TrimSpace(title)
	message = strings.TrimSpace(message)
	targetPaneID = strings.TrimSpace(targetPaneID)
	clearCommand = strings.TrimSpace(clearCommand)

	if title == "" {
		return errors.New("title is required")
	}
	if message == "" {
		return errors.New("message is required")
	}
	if targetPaneID == "" {
		return errors.New("target pane id is required")
	}
	if err := s.deps.CheckPaneAlive(targetPaneID); err != nil {
		return fmt.Errorf("target pane unavailable: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	s.mu.Lock()
	if err := s.ensureNotRetiredLocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	item := QueueItem{
		ID:           uuid.New().String(),
		Title:        title,
		Message:      message,
		TargetPaneID: targetPaneID,
		OrderIndex:   len(s.items),
		Status:       ItemStatusPending,
		CreatedAt:    now,
		ClearBefore:  clearBefore,
		ClearCommand: clearCommand,
	}
	s.items = append(s.items, item)
	s.mu.Unlock()

	s.emitUpdated()
	return nil
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
	found := false
	filtered := make([]QueueItem, 0, len(s.items))
	for _, item := range s.items {
		if item.ID == id {
			if !IsEditable(item.Status) {
				s.mu.Unlock()
				return fmt.Errorf("cannot remove item %s with status %s", id, item.Status)
			}
			found = true
			continue
		}
		filtered = append(filtered, item)
	}
	if !found {
		s.mu.Unlock()
		return fmt.Errorf("item %s not found", id)
	}
	// Reindex.
	for i := range filtered {
		filtered[i].OrderIndex = i
	}
	s.items = filtered
	s.mu.Unlock()

	s.emitUpdated()
	return nil
}

// ReorderItems reorders items by their IDs.
func (s *Service) ReorderItems(orderedIDs []string) error {
	if len(orderedIDs) == 0 {
		return errors.New("ordered ids is required")
	}

	// Check for duplicate IDs before acquiring the lock.
	seen := make(map[string]struct{}, len(orderedIDs))
	for _, id := range orderedIDs {
		if _, dup := seen[id]; dup {
			return fmt.Errorf("duplicate id %s in ordered ids", id)
		}
		seen[id] = struct{}{}
	}

	s.mu.Lock()
	if err := s.ensureNotRetiredLocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	itemMap := make(map[string]QueueItem, len(s.items))
	for _, item := range s.items {
		itemMap[item.ID] = item
	}

	if len(orderedIDs) != len(itemMap) {
		s.mu.Unlock()
		return fmt.Errorf("ordered ids count %d does not match items count %d", len(orderedIDs), len(itemMap))
	}

	reordered := make([]QueueItem, 0, len(orderedIDs))
	for i, id := range orderedIDs {
		item, ok := itemMap[id]
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

// UpdateItem updates a non-running item's fields. Non-pending items are reset to pending.
func (s *Service) UpdateItem(id, title, message, targetPaneID string, clearBefore bool, clearCommand string) error {
	id = strings.TrimSpace(id)
	title = strings.TrimSpace(title)
	message = strings.TrimSpace(message)
	targetPaneID = strings.TrimSpace(targetPaneID)
	clearCommand = strings.TrimSpace(clearCommand)

	if id == "" {
		return errors.New("item id is required")
	}
	if title == "" {
		return errors.New("title is required")
	}
	if message == "" {
		return errors.New("message is required")
	}
	if targetPaneID == "" {
		return errors.New("target pane id is required")
	}
	if err := s.deps.CheckPaneAlive(targetPaneID); err != nil {
		return fmt.Errorf("target pane unavailable: %w", err)
	}

	s.mu.Lock()
	if err := s.ensureNotRetiredLocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	found := false
	for i := range s.items {
		if s.items[i].ID == id {
			if !IsEditable(s.items[i].Status) {
				s.mu.Unlock()
				return fmt.Errorf("cannot update item %s with status %s", id, s.items[i].Status)
			}
			// Reset non-pending items to pending so they can be re-executed.
			if s.items[i].Status != ItemStatusPending {
				s.items[i].Status = ItemStatusPending
				s.items[i].ErrorMessage = ""
				s.items[i].StartedAt = ""
				s.items[i].CompletedAt = ""
				s.items[i].OrcTaskID = ""
			}
			s.items[i].Title = title
			s.items[i].Message = message
			s.items[i].TargetPaneID = targetPaneID
			s.items[i].ClearBefore = clearBefore
			s.items[i].ClearCommand = clearCommand
			found = true
			break
		}
	}
	if !found {
		s.mu.Unlock()
		return fmt.Errorf("item %s not found", id)
	}
	s.mu.Unlock()

	s.emitUpdated()
	return nil
}

// GetStatus returns the current queue status.
// Returns a default idle snapshot when the service has been retired.
func (s *Service) GetStatus() QueueStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.retired {
		return QueueStatus{
			Items:        []QueueItem{},
			RunStatus:    QueueIdle,
			CurrentIndex: -1,
		}
	}

	items := make([]QueueItem, len(s.items))
	copy(items, s.items)

	return QueueStatus{
		Config:          s.config,
		Items:           items,
		RunStatus:       s.runStatus,
		CurrentIndex:    s.currentIndex,
		SessionName:     s.sessionName,
		GenerationID:    s.generationID,
		PreExecProgress: s.preExecProgress,
	}
}

// RenameSession rebinds the service to a renamed session without discarding
// the existing queue state.
func (s *Service) RenameSession(newSessionName string) error {
	newSessionName = strings.TrimSpace(newSessionName)
	if newSessionName == "" {
		return errors.New("new session name is required")
	}

	// Serialize renames with the initial DB bind so the winner of the race
	// decides which session name the handle is opened for.
	s.dbMu.Lock()
	defer s.dbMu.Unlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureNotRetiredLocked(); err != nil {
		return err
	}
	s.sessionName = newSessionName
	return nil
}

// ------------------------------------------------------------
// Retirement
// ------------------------------------------------------------

// Retire suppresses future frontend-facing events from a service that has been
// detached from the manager. External callers are rejected after retirement.
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

// ------------------------------------------------------------
// Worker
// ------------------------------------------------------------

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
	s.deps.LaunchWorker("task-scheduler", ctx, func(ctx context.Context) {
		s.mu.Lock()
		if !s.isCurrentGenerationLocked(generationID) {
			s.mu.Unlock()
			return
		}
		items := make([]QueueItem, len(s.items))
		copy(items, s.items)
		config := s.config
		s.mu.Unlock()

		if config.PreExecEnabled {
			if !s.runPreExecutionPhase(ctx, generationID, items, config) {
				return
			}

			s.mu.Lock()
			if s.runStatus != QueuePreparing || !s.isCurrentGenerationLocked(generationID) {
				s.mu.Unlock()
				return
			}
			s.runStatus = QueueRunning
			s.currentIndex = -1
			s.mu.Unlock()
			s.emitUpdated()
		}

		s.runLoop(ctx, generationID)
	}, recoveryOpts)
}

func (s *Service) runLoop(ctx context.Context, generationID string) {
	waitingForTaskIDItemID := ""
	waitingForTaskIDIdx := -1
	waitingForTaskIDSince := time.Time{}

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

		// First check for a running item that needs resume (after Pause/Resume).
		nextIdx := -1
		resuming := false
		waitingForTaskID := false
		currentWaitingForTaskIDItemID := ""
		currentWaitingForTaskIDIdx := -1
		currentWaitingForTaskIDTaskID := ""
		for i := range s.items {
			if s.items[i].Status != ItemStatusRunning {
				continue
			}
			if s.items[i].OrcTaskID != "" {
				nextIdx = i
				resuming = true
				break
			}
			waitingForTaskID = true
			currentWaitingForTaskIDIdx = i
			currentWaitingForTaskIDItemID = s.items[i].ID
			currentWaitingForTaskIDTaskID = s.pendingOrcTaskIDs[s.items[i].ID]
		}

		// Then find the next pending item.
		if nextIdx == -1 && !waitingForTaskID {
			for i := range s.items {
				if s.items[i].Status == ItemStatusPending {
					nextIdx = i
					break
				}
			}
		}

		if nextIdx == -1 {
			if waitingForTaskID {
				if currentWaitingForTaskIDIdx == -1 || currentWaitingForTaskIDItemID == "" {
					waitingForTaskIDSince = time.Time{}
				} else if currentWaitingForTaskIDItemID != waitingForTaskIDItemID || currentWaitingForTaskIDIdx != waitingForTaskIDIdx {
					waitingForTaskIDSince = time.Now()
				}
				if waitingForTaskIDSince.IsZero() {
					waitingForTaskIDSince = time.Now()
				}
				if waitingTaskIDTimedOut(waitingForTaskIDSince, time.Now()) {
					timedOutIdx := currentWaitingForTaskIDIdx
					timedOutItemID := currentWaitingForTaskIDItemID
					timedOutTaskID := currentWaitingForTaskIDTaskID
					delete(s.pendingOrcTaskIDs, timedOutItemID)
					s.mu.Unlock()

					reason := fmt.Sprintf("timed out waiting for orchestrator task id after %s", staleTaskIDWaitTimeout)
					if timedOutTaskID != "" {
						if err := s.orcDBAbandonPendingTask(timedOutTaskID); err != nil {
							reason = fmt.Sprintf("%s (cleanup failed: %v)", reason, err)
						}
					}
					s.failUndeliveredItem(generationID, timedOutIdx, timedOutItemID, reason)
					return
				}
				waitingForTaskIDItemID = currentWaitingForTaskIDItemID
				waitingForTaskIDIdx = currentWaitingForTaskIDIdx
				s.mu.Unlock()
				if !waitForTaskID(ctx) {
					return
				}
				continue
			}
			waitingForTaskIDItemID = ""
			waitingForTaskIDIdx = -1
			waitingForTaskIDSince = time.Time{}
			// All items processed.
			s.runStatus = QueueCompleted
			s.currentIndex = -1
			s.preExecProgress = ""
			s.mu.Unlock()
			s.closeOrcDB()
			s.emitUpdated()
			slog.Info("[DEBUG-TASK-SCHEDULER] queue completed")
			return
		}
		waitingForTaskIDItemID = ""
		waitingForTaskIDIdx = -1
		waitingForTaskIDSince = time.Time{}

		s.currentIndex = nextIdx
		item := s.items[nextIdx]
		s.mu.Unlock()

		s.emitUpdated()

		if resuming {
			if !s.resumeRunningItem(ctx, generationID, nextIdx, item) {
				return
			}
		} else {
			if !s.executeItem(ctx, generationID, nextIdx, item) {
				return
			}
		}
	}
}

// executeItem executes a single queue item. Returns true if the item completed
// successfully and the worker should continue, false if it failed and the
// worker should stop.
func (s *Service) executeItem(ctx context.Context, generationID string, idx int, item QueueItem) bool {
	// Mark item as running.
	now := time.Now().UTC().Format(time.RFC3339)
	s.mu.Lock()
	if !s.isCurrentGenerationLocked(generationID) {
		s.mu.Unlock()
		return false
	}
	s.items[idx].Status = ItemStatusRunning
	s.items[idx].StartedAt = now
	s.mu.Unlock()
	s.emitUpdated()

	// Ensure orchestrator DB connection.
	if err := s.ensureOrcDB(); err != nil {
		s.failItemBeforeTaskID(generationID, idx, item.ID, fmt.Sprintf("orchestrator db: %v", err))
		return false
	}

	// Check pane alive before sending.
	if err := s.deps.CheckPaneAlive(item.TargetPaneID); err != nil {
		s.failItemBeforeTaskID(generationID, idx, item.ID, fmt.Sprintf("target pane unavailable: %v", err))
		return false
	}

	// Execute pre-task clear command if configured.
	if !s.executeClearPreStep(ctx, generationID, idx, item) {
		return false
	}

	// Register task-master agent and create task in orchestrator DB.
	if err := s.orcDBEnsureTaskMaster(); err != nil {
		s.failItemBeforeTaskID(generationID, idx, item.ID, fmt.Sprintf("register task-master: %v", err))
		return false
	}

	taskID, err := s.orcDBCreateTask(item.TargetPaneID, item.Message)
	if err != nil {
		s.failItemBeforeTaskID(generationID, idx, item.ID, fmt.Sprintf("create orchestrator task: %v", err))
		return false
	}
	s.setPendingOrcTaskID(idx, item.ID, taskID)
	if err := ctx.Err(); err != nil {
		reason := "task could not be sent after cancellation"
		if abandonErr := s.orcDBAbandonPendingTask(taskID); abandonErr != nil {
			slog.Warn("[WARN-TASK-SCHEDULER] abandon undelivered task after cancellation",
				"taskID", taskID, "error", abandonErr)
			reason = fmt.Sprintf("%s (cleanup failed: %v)", reason, abandonErr)
		}
		s.failUndeliveredItem(generationID, idx, item.ID, reason)
		return false
	}

	// Build and send the message with response instruction.
	fullMessage := item.Message + "\n\n---\n" + buildTaskResponseInstruction(taskID)
	if err := s.deps.SendMessagePaste(item.TargetPaneID, fullMessage); err != nil {
		reason := fmt.Sprintf("send message: %v", err)
		if abandonErr := s.orcDBAbandonPendingTask(taskID); abandonErr != nil {
			reason = fmt.Sprintf("%s (cleanup failed: %v)", reason, abandonErr)
		}
		s.failUndeliveredItem(generationID, idx, item.ID, reason)
		return false
	}
	if err := ctx.Err(); err != nil && !s.canBackfillTaskID(idx, item.ID) {
		reason := "task could not be finalized after cancellation"
		if abandonErr := s.orcDBAbandonPendingTask(taskID); abandonErr != nil {
			reason = fmt.Sprintf("%s (cleanup failed: %v)", reason, abandonErr)
		}
		s.failUndeliveredItem(generationID, idx, item.ID, reason)
		return false
	}
	s.setItemOrcTaskID(idx, item.ID, taskID)

	slog.Info("[DEBUG-TASK-SCHEDULER] task sent",
		"itemID", item.ID, "taskID", taskID, "pane", item.TargetPaneID)

	// Poll for completion.
	if err := s.pollForCompletion(ctx, taskID); err != nil {
		// Context cancellation is not a failure — it means stop/shutdown was requested.
		if ctx.Err() != nil {
			return false
		}
		s.failItem(generationID, idx, fmt.Sprintf("completion poll: %v", err))
		return false
	}

	// Mark item completed.
	completedAt := time.Now().UTC().Format(time.RFC3339)
	s.mu.Lock()
	if !s.isCurrentGenerationLocked(generationID) {
		s.mu.Unlock()
		return false
	}
	s.items[idx].Status = ItemStatusCompleted
	s.items[idx].CompletedAt = completedAt
	s.mu.Unlock()
	s.emitUpdated()

	slog.Info("[DEBUG-TASK-SCHEDULER] task completed",
		"itemID", item.ID, "taskID", taskID)

	return true
}

// resumeRunningItem resumes polling for an item that was already sent to a pane
// (e.g. after Pause → Resume). Returns true if the item completed successfully.
func (s *Service) resumeRunningItem(ctx context.Context, generationID string, idx int, item QueueItem) bool {
	slog.Info("[DEBUG-TASK-SCHEDULER] resuming poll for running item",
		"itemID", item.ID, "taskID", item.OrcTaskID, "pane", item.TargetPaneID)

	if err := s.ensureOrcDB(); err != nil {
		s.failItem(generationID, idx, fmt.Sprintf("orchestrator db (resume): %v", err))
		return false
	}

	if err := s.pollForCompletion(ctx, item.OrcTaskID); err != nil {
		if ctx.Err() != nil {
			return false
		}
		s.failItem(generationID, idx, fmt.Sprintf("completion poll (resume): %v", err))
		return false
	}

	completedAt := time.Now().UTC().Format(time.RFC3339)
	s.mu.Lock()
	if !s.isCurrentGenerationLocked(generationID) {
		s.mu.Unlock()
		return false
	}
	s.items[idx].Status = ItemStatusCompleted
	s.items[idx].CompletedAt = completedAt
	s.mu.Unlock()
	s.emitUpdated()

	slog.Info("[DEBUG-TASK-SCHEDULER] resumed task completed",
		"itemID", item.ID, "taskID", item.OrcTaskID)

	return true
}

func (s *Service) pollForCompletion(ctx context.Context, taskID string) error {
	return s.pollForCompletionWithInterval(ctx, taskID, pollInterval)
}

func (s *Service) pollForCompletionWithInterval(ctx context.Context, taskID string, interval time.Duration) error {
	if interval <= 0 {
		interval = pollInterval
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	consecutivePollErrors := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			status, _, err := s.orcDBPollTaskStatus(taskID)
			if err != nil {
				consecutivePollErrors++
				slog.Warn("[WARN-TASK-SCHEDULER] poll error",
					"taskID", taskID,
					"attempt", consecutivePollErrors,
					"maxAttempts", maxConsecutivePollErrors,
					"error", err)
				if consecutivePollErrors >= maxConsecutivePollErrors {
					return fmt.Errorf("poll task %s failed after %d attempts: %w", taskID, consecutivePollErrors, err)
				}
				continue
			}
			consecutivePollErrors = 0
			switch classifyOrchestratorTaskStatus(status) {
			case orchestratorTaskOutcomeCompleted:
				return nil
			case orchestratorTaskOutcomeFailed:
				return fmt.Errorf("task %s ended with status: %s", taskID, status)
			default:
				// Still pending — continue polling.
			}
		}
	}
}

type orchestratorTaskOutcome int

const (
	orchestratorTaskOutcomePolling orchestratorTaskOutcome = iota
	orchestratorTaskOutcomeCompleted
	orchestratorTaskOutcomeFailed
)

func classifyOrchestratorTaskStatus(status orcdomain.TaskStatus) orchestratorTaskOutcome {
	switch status {
	case orcdomain.TaskStatusPending,
		orcdomain.TaskStatusBlocked:
		return orchestratorTaskOutcomePolling
	case orcdomain.TaskStatusCompleted:
		return orchestratorTaskOutcomeCompleted
	case orcdomain.TaskStatusFailed,
		orcdomain.TaskStatusAbandoned,
		orcdomain.TaskStatusCancelled,
		orcdomain.TaskStatusExpired:
		return orchestratorTaskOutcomeFailed
	default:
		slog.Warn("[WARN-TASK-SCHEDULER] unknown orchestrator task status treated as failure",
			"status", status)
		return orchestratorTaskOutcomeFailed
	}
}

// executeClearPreStep sends a clear command to the target pane before task
// execution. The clear step is a strict precondition: send failures mark the
// item as failed instead of continuing with stale context. Returns true if the
// caller should continue, false if the precondition failed or the context was
// cancelled.
func (s *Service) executeClearPreStep(ctx context.Context, generationID string, idx int, item QueueItem) bool {
	if !item.ClearBefore {
		return true
	}

	clearCmd := item.ClearCommand
	if clearCmd == "" {
		clearCmd = defaultClearCommand
	}

	if err := s.deps.SendClearCommand(item.TargetPaneID, clearCmd); err != nil {
		slog.Warn("[WARN-TASK-SCHEDULER] clear command failed",
			"pane", item.TargetPaneID, "cmd", clearCmd, "error", err)
		s.failItemBeforeTaskID(generationID, idx, item.ID, fmt.Sprintf("clear command: %v", err))
		return false
	}

	// Wait for the clear command to be processed.
	select {
	case <-ctx.Done():
		return false
	case <-time.After(clearCommandDelay):
	}
	if !s.isCurrentGeneration(generationID) {
		return false
	}
	return true
}

// ------------------------------------------------------------
// Internal helpers
// ------------------------------------------------------------

// failItem marks the item as failed and transitions the queue to idle.
func (s *Service) failItem(generationID string, idx int, reason string) {
	now := time.Now().UTC().Format(time.RFC3339)
	eventSessionName := ""
	eventGenerationID := ""
	s.mu.Lock()
	if !s.isCurrentGenerationLocked(generationID) {
		currentGenerationID := s.generationID
		currentRunStatus := s.runStatus
		s.mu.Unlock()
		slog.Warn("[WARN-TASK-SCHEDULER] ignored stale generation failure",
			"idx", idx,
			"staleGenerationID", generationID,
			"currentGenerationID", currentGenerationID,
			"currentRunStatus", currentRunStatus,
			"reason", reason)
		return
	}
	if idx >= 0 && idx < len(s.items) {
		delete(s.pendingOrcTaskIDs, s.items[idx].ID)
		s.items[idx].Status = ItemStatusFailed
		s.items[idx].ErrorMessage = reason
		s.items[idx].CompletedAt = now
	}
	s.runStatus = QueueIdle
	s.currentIndex = -1
	s.preExecProgress = ""
	eventSessionName = s.sessionName
	eventGenerationID = s.generationID
	s.mu.Unlock()

	s.closeOrcDB()
	slog.Warn("[WARN-TASK-SCHEDULER] item failed", "idx", idx, "reason", reason)
	s.emitStopped(reason, eventSessionName, eventGenerationID)
	s.emitUpdated()
}

// failItemBeforeTaskID fails a running item before the first orchestrator task
// ID is attached. A stale worker may still finalize the same item after Resume
// rotates the generation, as long as the same queue item is still running and
// waiting for its first task ID. When that stale-generation bypass is used, the
// stopped event is emitted with the current generation so the frontend can
// observe a terminal event for the active resumed run.
func (s *Service) failItemBeforeTaskID(generationID string, idx int, itemID, reason string) {
	now := time.Now().UTC().Format(time.RFC3339)
	eventSessionName := ""
	eventGenerationID := ""
	staleGeneration := false
	s.mu.Lock()
	if idx < 0 || idx >= len(s.items) || s.items[idx].ID != itemID {
		s.mu.Unlock()
		return
	}
	delete(s.pendingOrcTaskIDs, itemID)
	if s.items[idx].Status != ItemStatusRunning || s.items[idx].OrcTaskID != "" {
		s.mu.Unlock()
		return
	}
	if !s.isCurrentGenerationLocked(generationID) {
		staleGeneration = true
	}
	s.items[idx].Status = ItemStatusFailed
	s.items[idx].ErrorMessage = reason
	s.items[idx].CompletedAt = now
	s.runStatus = QueueIdle
	s.currentIndex = -1
	s.preExecProgress = ""
	eventSessionName = s.sessionName
	eventGenerationID = s.generationID
	s.mu.Unlock()

	s.closeOrcDB()
	slog.Warn("[WARN-TASK-SCHEDULER] item failed before send",
		"idx", idx, "itemID", itemID, "reason", reason, "stale_generation", staleGeneration)
	s.emitStopped(reason, eventSessionName, eventGenerationID)
	s.emitUpdated()
}

// failUndeliveredItem fails a task that was registered in the orchestrator DB
// but never received its first task ID update in the queue state.
func (s *Service) failUndeliveredItem(generationID string, idx int, itemID, reason string) {
	s.failItemBeforeTaskID(generationID, idx, itemID, reason)
}

func (s *Service) handleWorkerFatal(generationID string, reason string) {
	eventSessionName := ""
	eventGenerationID := ""
	s.mu.Lock()
	if !s.isCurrentGenerationLocked(generationID) {
		currentGenerationID := s.generationID
		currentRunStatus := s.runStatus
		s.mu.Unlock()
		slog.Warn("[WARN-TASK-SCHEDULER] ignored stale worker fatal",
			"staleGenerationID", generationID,
			"currentGenerationID", currentGenerationID,
			"currentRunStatus", currentRunStatus,
			"reason", reason)
		return
	}
	s.pendingOrcTaskIDs = make(map[string]string)
	s.runStatus = QueueIdle
	s.currentIndex = -1
	s.preExecProgress = ""
	eventSessionName = s.sessionName
	eventGenerationID = s.generationID
	s.mu.Unlock()

	s.closeOrcDB()
	s.emitStopped(reason, eventSessionName, eventGenerationID)
	s.emitUpdated()
}

func (s *Service) isCurrentGeneration(generationID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isCurrentGenerationLocked(generationID)
}

func (s *Service) isCurrentGenerationLocked(generationID string) bool {
	return s.generationID == generationID
}

// orcDBEnsureTaskMaster wraps the DB call with dbMu protection.
func (s *Service) orcDBEnsureTaskMaster() error {
	s.dbMu.Lock()
	defer s.dbMu.Unlock()
	if s.orcDB == nil {
		return errors.New("orchestrator db not open")
	}
	return s.orcDB.ensureTaskMasterAgent()
}

// orcDBCreateTask wraps the DB call with dbMu protection.
func (s *Service) orcDBCreateTask(assigneePaneID, message string) (string, error) {
	s.dbMu.Lock()
	defer s.dbMu.Unlock()
	if s.orcDB == nil {
		return "", errors.New("orchestrator db not open")
	}
	taskID, _, err := s.orcDB.createTask(assigneePaneID, message)
	return taskID, err
}

// orcDBPollTaskStatus wraps the DB call with dbMu protection.
func (s *Service) orcDBPollTaskStatus(taskID string) (orcdomain.TaskStatus, string, error) {
	s.dbMu.Lock()
	defer s.dbMu.Unlock()
	if s.orcDB == nil {
		return "", "", errors.New("orchestrator db not open")
	}
	return s.orcDB.pollTaskStatus(taskID)
}

func (s *Service) ensureOrcDB() error {
	s.dbMu.Lock()
	defer s.dbMu.Unlock()
	if s.orcDB != nil {
		return nil
	}
	// Snapshot the session name only after dbMu is held so RenameSession and the
	// first DB open are serialized by the same lock.
	sessionName := s.sessionNameSnapshot()
	dbPath, err := s.deps.ResolveOrchestratorDBPath(sessionName)
	if err != nil {
		return fmt.Errorf("resolve db path: %w", err)
	}
	db, err := openOrchestratorDB(dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	s.orcDB = db
	return nil
}

func (s *Service) closeOrcDB() {
	s.dbMu.Lock()
	defer s.dbMu.Unlock()
	if s.orcDB != nil {
		if err := s.orcDB.Close(); err != nil {
			slog.Warn("[WARN-TASK-SCHEDULER] close orchestrator db", "error", err)
		}
		s.orcDB = nil
	}
}

func waitForTaskID(ctx context.Context) bool {
	timer := time.NewTimer(staleTaskIDWaitInterval)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func waitingTaskIDTimedOut(waitSince, now time.Time) bool {
	if waitSince.IsZero() {
		return false
	}
	return !now.Before(waitSince.Add(staleTaskIDWaitTimeout))
}

func (s *Service) setPendingOrcTaskID(idx int, itemID, taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if idx < 0 || idx >= len(s.items) || s.items[idx].ID != itemID {
		return
	}
	if s.items[idx].Status != ItemStatusRunning || s.items[idx].OrcTaskID != "" {
		return
	}
	s.pendingOrcTaskIDs[itemID] = taskID
}

func (s *Service) canBackfillTaskID(idx int, itemID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if idx < 0 || idx >= len(s.items) || s.items[idx].ID != itemID {
		return false
	}
	return s.items[idx].Status == ItemStatusRunning && s.items[idx].OrcTaskID == ""
}

func (s *Service) setItemOrcTaskID(idx int, itemID, taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if idx < 0 || idx >= len(s.items) {
		return
	}
	if s.items[idx].ID != itemID {
		return
	}
	delete(s.pendingOrcTaskIDs, itemID)
	if s.items[idx].OrcTaskID != "" {
		return
	}
	if s.items[idx].Status != ItemStatusRunning {
		return
	}
	s.items[idx].OrcTaskID = taskID
}

// orcDBAbandonPendingTask wraps the DB call with dbMu protection.
func (s *Service) orcDBAbandonPendingTask(taskID string) error {
	s.dbMu.Lock()
	defer s.dbMu.Unlock()
	if s.orcDB == nil {
		return errors.New("orchestrator db not open")
	}
	return s.orcDB.abandonPendingTask(taskID)
}

func (s *Service) collectTrackedOrchestratorTaskIDsLocked() []string {
	seen := make(map[string]struct{})
	taskIDs := make([]string, 0, len(s.items)+len(s.pendingOrcTaskIDs))
	for i := range s.items {
		taskID := strings.TrimSpace(s.items[i].OrcTaskID)
		if taskID == "" {
			continue
		}
		if _, ok := seen[taskID]; ok {
			continue
		}
		seen[taskID] = struct{}{}
		taskIDs = append(taskIDs, taskID)
	}
	for _, taskID := range s.pendingOrcTaskIDs {
		taskID = strings.TrimSpace(taskID)
		if taskID == "" {
			continue
		}
		if _, ok := seen[taskID]; ok {
			continue
		}
		seen[taskID] = struct{}{}
		taskIDs = append(taskIDs, taskID)
	}
	return taskIDs
}

func (s *Service) abandonTrackedTasks(taskIDs []string) error {
	var cleanupErr error
	for _, taskID := range taskIDs {
		if strings.TrimSpace(taskID) == "" {
			continue
		}
		if err := s.orcDBAbandonPendingTask(taskID); err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("abandon task %s: %w", taskID, err))
		}
	}
	return cleanupErr
}

// ------------------------------------------------------------
// Event emission
// ------------------------------------------------------------

func (s *Service) emitUpdated() {
	if !s.shouldEmit() {
		return
	}
	s.deps.Emitter.Emit("task-scheduler:updated", s.GetStatus())
}

func (s *Service) emitStopped(reason string, sessionName string, generationID string) {
	if !s.shouldEmit() {
		return
	}
	s.deps.Emitter.Emit("task-scheduler:stopped", map[string]string{
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

// ------------------------------------------------------------
// Message formatting
// ------------------------------------------------------------

func buildTaskResponseInstruction(taskID string) string {
	if taskID == "" {
		taskID = "<task_id>"
	}
	return "【タスク完了時の応答方法】\n" +
		"このタスクが完了したら、send_response MCPツールで完了を報告してください。\n" +
		"task_id=" + taskID + "\n" +
		"send_response(task_id=\"" + taskID + "\", message=\"完了報告: [結果の概要]\")\n" +
		"task_id が抜けるとタスクを完了できません。"
}
