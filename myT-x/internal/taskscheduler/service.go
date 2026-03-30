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
)

const (
	// pollInterval is how often the worker checks for task completion.
	pollInterval = 10 * time.Second

	// defaultClearCommand is the default command sent to clear context before a task.
	defaultClearCommand = "/new"

	// clearCommandDelay is the wait time after sending a clear command.
	// 2 seconds is sufficient for typical AI tool commands like /new to be
	// accepted and processed before the next task message is sent.
	clearCommandDelay = 2 * time.Second
)

// Service manages the task scheduler queue, execution, and completion detection.
//
// Thread-safety is managed internally via mu (queue state) and dbMu
// (orchestrator DB access). No external locking is required.
type Service struct {
	deps Deps
	mu   sync.Mutex

	// Queue state (protected by mu).
	items           []QueueItem
	config          QueueConfig
	runStatus       QueueRunStatus
	currentIndex    int    // -1 when not running
	preExecProgress string // Empty when not in the pre-execution phase.

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
		deps:         deps,
		items:        []QueueItem{},
		runStatus:    QueueIdle,
		currentIndex: -1,
	}
}

// ------------------------------------------------------------
// Queue lifecycle
// ------------------------------------------------------------

// Start begins executing the task queue with the given config and items.
func (s *Service) Start(config QueueConfig, items []QueueItem) error {
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
	if s.runStatus == QueueRunning || s.runStatus == QueuePaused || s.runStatus == QueuePreparing {
		s.mu.Unlock()
		return errors.New("queue is already running; stop it first")
	}
	s.items = prepared
	s.config = config
	s.preExecProgress = ""
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

	s.launchWorker(ctx)
	s.emitUpdated()

	slog.Info("[DEBUG-TASK-SCHEDULER] started", "itemCount", len(prepared))
	return nil
}

// Stop stops the queue and marks remaining items as skipped.
func (s *Service) Stop() error {
	s.mu.Lock()
	if s.runStatus != QueueRunning && s.runStatus != QueuePaused && s.runStatus != QueuePreparing {
		s.mu.Unlock()
		return errors.New("queue is not running")
	}
	cancel := s.cancel
	s.cancel = nil
	for i := range s.items {
		if !IsTerminal(s.items[i].Status) {
			s.items[i].Status = ItemStatusSkipped
		}
	}
	s.runStatus = QueueIdle
	s.currentIndex = -1
	s.preExecProgress = ""
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	s.closeOrcDB()
	s.emitUpdated()

	slog.Info("[DEBUG-TASK-SCHEDULER] stopped")
	return nil
}

// Pause pauses an actively running queue by cancelling the current worker
// context. The preparing phase is intentionally not pausable because it does
// not have resumable intermediate state.
// If an item is currently running, its status is preserved as "running".
// Resume() detects the running item and re-polls for its completion.
func (s *Service) Pause() error {
	s.mu.Lock()
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
	if s.runStatus != QueuePaused {
		s.mu.Unlock()
		return errors.New("queue is not paused")
	}
	s.runStatus = QueueRunning
	s.mu.Unlock()

	ctx, cancel := s.deps.NewContext()

	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()

	s.launchWorker(ctx)
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
	s.runStatus = QueueIdle
	s.currentIndex = -1
	s.preExecProgress = ""
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	s.closeOrcDB()
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

	now := time.Now().UTC().Format(time.RFC3339)

	s.mu.Lock()
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

	s.mu.Lock()
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
func (s *Service) GetStatus() QueueStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	items := make([]QueueItem, len(s.items))
	copy(items, s.items)

	return QueueStatus{
		Config:          s.config,
		Items:           items,
		RunStatus:       s.runStatus,
		CurrentIndex:    s.currentIndex,
		SessionName:     s.deps.SessionName,
		PreExecProgress: s.preExecProgress,
	}
}

// ------------------------------------------------------------
// Worker
// ------------------------------------------------------------

func (s *Service) launchWorker(ctx context.Context) {
	recoveryOpts := s.deps.BaseRecoveryOptions()
	recoveryOpts.MaxRetries = 1
	origOnFatal := recoveryOpts.OnFatal
	recoveryOpts.OnFatal = func(worker string, maxRetries int) {
		s.handleWorkerFatal("internal panic")
		if origOnFatal != nil {
			origOnFatal(worker, maxRetries)
		}
	}
	s.deps.LaunchWorker("task-scheduler", ctx, func(ctx context.Context) {
		s.mu.Lock()
		items := make([]QueueItem, len(s.items))
		copy(items, s.items)
		config := s.config
		s.mu.Unlock()

		if config.PreExecEnabled {
			if !s.runPreExecutionPhase(ctx, items, config) {
				return
			}

			s.mu.Lock()
			if s.runStatus != QueuePreparing {
				s.mu.Unlock()
				return
			}
			s.runStatus = QueueRunning
			s.currentIndex = -1
			s.mu.Unlock()
			s.emitUpdated()
		}

		s.runLoop(ctx)
	}, recoveryOpts)
}

func (s *Service) runLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		s.mu.Lock()
		if s.runStatus != QueueRunning {
			s.mu.Unlock()
			return
		}

		// First check for a running item that needs resume (after Pause/Resume).
		nextIdx := -1
		resuming := false
		for i := range s.items {
			if s.items[i].Status == ItemStatusRunning && s.items[i].OrcTaskID != "" {
				nextIdx = i
				resuming = true
				break
			}
		}

		// Then find the next pending item.
		if nextIdx == -1 {
			for i := range s.items {
				if s.items[i].Status == ItemStatusPending {
					nextIdx = i
					break
				}
			}
		}

		if nextIdx == -1 {
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

		s.currentIndex = nextIdx
		item := s.items[nextIdx]
		s.mu.Unlock()

		s.emitUpdated()

		if resuming {
			if !s.resumeRunningItem(ctx, nextIdx, item) {
				return
			}
		} else {
			if !s.executeItem(ctx, nextIdx, item) {
				return
			}
		}
	}
}

// executeItem executes a single queue item. Returns true if the item completed
// successfully and the worker should continue, false if it failed and the
// worker should stop.
func (s *Service) executeItem(ctx context.Context, idx int, item QueueItem) bool {
	// Mark item as running.
	now := time.Now().UTC().Format(time.RFC3339)
	s.mu.Lock()
	s.items[idx].Status = ItemStatusRunning
	s.items[idx].StartedAt = now
	s.mu.Unlock()
	s.emitUpdated()

	// Ensure orchestrator DB connection.
	if err := s.ensureOrcDB(); err != nil {
		s.failItem(idx, fmt.Sprintf("orchestrator db: %v", err))
		return false
	}

	// Check pane alive before sending.
	if err := s.deps.CheckPaneAlive(item.TargetPaneID); err != nil {
		s.failItem(idx, fmt.Sprintf("target pane unavailable: %v", err))
		return false
	}

	// Execute pre-task clear command if configured.
	if !s.executeClearPreStep(ctx, item) {
		return false
	}

	// Register task-master agent and create task in orchestrator DB.
	if err := s.orcDBEnsureTaskMaster(); err != nil {
		s.failItem(idx, fmt.Sprintf("register task-master: %v", err))
		return false
	}

	taskID, err := s.orcDBCreateTask(item.TargetPaneID, item.Message)
	if err != nil {
		s.failItem(idx, fmt.Sprintf("create orchestrator task: %v", err))
		return false
	}

	s.mu.Lock()
	s.items[idx].OrcTaskID = taskID
	s.mu.Unlock()

	// Build and send the message with response instruction.
	fullMessage := item.Message + "\n\n---\n" + buildTaskResponseInstruction(taskID)
	if err := s.deps.SendMessagePaste(item.TargetPaneID, fullMessage); err != nil {
		s.failItem(idx, fmt.Sprintf("send message: %v", err))
		return false
	}

	slog.Info("[DEBUG-TASK-SCHEDULER] task sent",
		"itemID", item.ID, "taskID", taskID, "pane", item.TargetPaneID)

	// Poll for completion.
	if err := s.pollForCompletion(ctx, taskID); err != nil {
		// Context cancellation is not a failure — it means stop/shutdown was requested.
		if ctx.Err() != nil {
			return false
		}
		s.failItem(idx, fmt.Sprintf("completion poll: %v", err))
		return false
	}

	// Mark item completed.
	completedAt := time.Now().UTC().Format(time.RFC3339)
	s.mu.Lock()
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
func (s *Service) resumeRunningItem(ctx context.Context, idx int, item QueueItem) bool {
	slog.Info("[DEBUG-TASK-SCHEDULER] resuming poll for running item",
		"itemID", item.ID, "taskID", item.OrcTaskID, "pane", item.TargetPaneID)

	if err := s.ensureOrcDB(); err != nil {
		s.failItem(idx, fmt.Sprintf("orchestrator db (resume): %v", err))
		return false
	}

	if err := s.pollForCompletion(ctx, item.OrcTaskID); err != nil {
		if ctx.Err() != nil {
			return false
		}
		s.failItem(idx, fmt.Sprintf("completion poll (resume): %v", err))
		return false
	}

	completedAt := time.Now().UTC().Format(time.RFC3339)
	s.mu.Lock()
	s.items[idx].Status = ItemStatusCompleted
	s.items[idx].CompletedAt = completedAt
	s.mu.Unlock()
	s.emitUpdated()

	slog.Info("[DEBUG-TASK-SCHEDULER] resumed task completed",
		"itemID", item.ID, "taskID", item.OrcTaskID)

	return true
}

func (s *Service) pollForCompletion(ctx context.Context, taskID string) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			status, _, err := s.orcDBPollTaskStatus(taskID)
			if err != nil {
				slog.Warn("[DEBUG-TASK-SCHEDULER] poll error", "taskID", taskID, "error", err)
				continue // Transient error — retry on next tick.
			}
			switch status {
			case "completed":
				return nil
			case "failed", "abandoned":
				return fmt.Errorf("task %s ended with status: %s", taskID, status)
			default:
				// Still pending — continue polling.
			}
		}
	}
}

// executeClearPreStep sends a clear command to the target pane before task execution.
// Returns true if the caller should continue, false if the context was cancelled.
func (s *Service) executeClearPreStep(ctx context.Context, item QueueItem) bool {
	if !item.ClearBefore {
		return true
	}

	clearCmd := item.ClearCommand
	if clearCmd == "" {
		clearCmd = defaultClearCommand
	}

	// Best-effort: clear command failure should not block task execution.
	// The clear command is a convenience pre-step, not a hard prerequisite.
	if err := s.deps.SendClearCommand(item.TargetPaneID, clearCmd); err != nil {
		slog.Warn("[DEBUG-TASK-SCHEDULER] clear command failed",
			"pane", item.TargetPaneID, "cmd", clearCmd, "error", err)
	}

	// Wait for the clear command to be processed.
	select {
	case <-ctx.Done():
		return false
	case <-time.After(clearCommandDelay):
	}
	return true
}

// ------------------------------------------------------------
// Internal helpers
// ------------------------------------------------------------

// failItem marks the item as failed and transitions the queue to idle.
func (s *Service) failItem(idx int, reason string) {
	now := time.Now().UTC().Format(time.RFC3339)
	s.mu.Lock()
	if idx < len(s.items) {
		s.items[idx].Status = ItemStatusFailed
		s.items[idx].ErrorMessage = reason
		s.items[idx].CompletedAt = now
	}
	s.runStatus = QueueIdle
	s.currentIndex = -1
	s.preExecProgress = ""
	s.mu.Unlock()

	slog.Warn("[DEBUG-TASK-SCHEDULER] item failed", "idx", idx, "reason", reason)
	s.emitStopped(reason)
	s.emitUpdated()
}

func (s *Service) handleWorkerFatal(reason string) {
	s.mu.Lock()
	s.runStatus = QueueIdle
	s.currentIndex = -1
	s.preExecProgress = ""
	s.mu.Unlock()

	s.closeOrcDB()
	s.emitStopped(reason)
	s.emitUpdated()
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
func (s *Service) orcDBPollTaskStatus(taskID string) (string, string, error) {
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
	dbPath, err := s.deps.ResolveOrchestratorDBPath()
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
			slog.Warn("[DEBUG-TASK-SCHEDULER] close orchestrator db", "error", err)
		}
		s.orcDB = nil
	}
}

// ------------------------------------------------------------
// Event emission
// ------------------------------------------------------------

func (s *Service) emitUpdated() {
	status := s.GetStatus()
	s.deps.Emitter.Emit("task-scheduler:updated", status)
}

func (s *Service) emitStopped(reason string) {
	s.deps.Emitter.Emit("task-scheduler:stopped", map[string]string{
		"reason":       reason,
		"session_name": s.deps.SessionName,
	})
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
