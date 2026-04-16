package main

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"myT-x/internal/orchestrator"
	"myT-x/internal/taskscheduler"
)

// TaskSchedulerQueueStatus is the frontend-safe representation of the queue status.
type TaskSchedulerQueueStatus = taskscheduler.QueueStatus

// TaskSchedulerQueueItem is the frontend-safe representation of a queue item.
type TaskSchedulerQueueItem = taskscheduler.QueueItem

// TaskSchedulerQueueConfig is the frontend-safe representation of queue config.
type TaskSchedulerQueueConfig = taskscheduler.QueueConfig

// taskSchedulerForActiveSession returns the Service for the active session.
func (a *App) taskSchedulerForActiveSession(sessionKey string) (*taskscheduler.Service, error) {
	sessionName, err := a.requireActiveSessionKey(sessionKey)
	if err != nil {
		return nil, err
	}
	return a.taskSchedulerManager.GetOrCreate(sessionName), nil
}

// GetTaskSchedulerStatus returns the current queue status.
// Wails-bound: called from the frontend task scheduler panel.
func (a *App) GetTaskSchedulerStatus(sessionKey string) (TaskSchedulerQueueStatus, error) {
	sessionName, hasSession, err := a.resolveOptionalSessionScopedRequest(sessionKey)
	if err != nil {
		return defaultTaskSchedulerQueueStatus(), err
	}
	if !hasSession {
		return defaultTaskSchedulerQueueStatus(), nil
	}
	return a.taskSchedulerManager.GetStatus(sessionName), nil
}

// StartTaskScheduler begins executing the task queue.
// Wails-bound: called from the frontend task scheduler panel.
func (a *App) StartTaskScheduler(sessionKey string, config TaskSchedulerQueueConfig, items []TaskSchedulerQueueItem) error {
	svc, err := a.taskSchedulerForActiveSession(sessionKey)
	if err != nil {
		return err
	}
	return svc.Start(config, items)
}

// StopTaskScheduler stops the queue and marks remaining items as skipped.
// Wails-bound: called from the frontend task scheduler panel.
func (a *App) StopTaskScheduler(sessionKey string) error {
	svc, err := a.taskSchedulerForActiveSession(sessionKey)
	if err != nil {
		return err
	}
	return svc.Stop()
}

// PauseTaskScheduler pauses the queue by cancelling the current worker.
// In-progress items are preserved and resumed when ResumeTaskScheduler is called.
// Wails-bound: called from the frontend task scheduler panel.
func (a *App) PauseTaskScheduler(sessionKey string) error {
	svc, err := a.taskSchedulerForActiveSession(sessionKey)
	if err != nil {
		return err
	}
	return svc.Pause()
}

// ResumeTaskScheduler resumes a paused queue.
// Wails-bound: called from the frontend task scheduler panel.
func (a *App) ResumeTaskScheduler(sessionKey string) error {
	svc, err := a.taskSchedulerForActiveSession(sessionKey)
	if err != nil {
		return err
	}
	return svc.Resume()
}

// AddTaskSchedulerItem adds a new task to the end of the queue.
// Wails-bound: called from the frontend task scheduler panel.
func (a *App) AddTaskSchedulerItem(sessionKey, title, message, targetPaneID string, clearBefore bool, clearCommand string) error {
	svc, err := a.taskSchedulerForActiveSession(sessionKey)
	if err != nil {
		return err
	}
	return svc.AddItem(title, message, targetPaneID, clearBefore, clearCommand)
}

// RemoveTaskSchedulerItem removes a non-running item from the queue.
// Wails-bound: called from the frontend task scheduler panel.
func (a *App) RemoveTaskSchedulerItem(sessionKey, id string) error {
	svc, err := a.taskSchedulerForActiveSession(sessionKey)
	if err != nil {
		return err
	}
	return svc.RemoveItem(id)
}

// ReorderTaskSchedulerItems reorders items by their IDs.
// Wails-bound: called from the frontend task scheduler panel.
func (a *App) ReorderTaskSchedulerItems(sessionKey string, orderedIDs []string) error {
	svc, err := a.taskSchedulerForActiveSession(sessionKey)
	if err != nil {
		return err
	}
	return svc.ReorderItems(orderedIDs)
}

// UpdateTaskSchedulerItem updates a non-running item's fields. Non-pending items are reset to pending.
// Wails-bound: called from the frontend task scheduler panel.
func (a *App) UpdateTaskSchedulerItem(sessionKey, id, title, message, targetPaneID string, clearBefore bool, clearCommand string) error {
	svc, err := a.taskSchedulerForActiveSession(sessionKey)
	if err != nil {
		return err
	}
	return svc.UpdateItem(id, title, message, targetPaneID, clearBefore, clearCommand)
}

// TaskSchedulerOrchestratorReadiness is the frontend-safe readiness result.
// It extends taskscheduler.OrchestratorReadiness with session-level pane state
// and the aggregate Ready flag used by the frontend flow.
type TaskSchedulerOrchestratorReadiness struct {
	Ready      bool `json:"ready"`
	DBExists   bool `json:"db_exists"`
	AgentCount int  `json:"agent_count"`
	HasPanes   bool `json:"has_panes"`
}

// CheckTaskSchedulerOrchestratorReady checks if the orchestrator is ready for task scheduling.
// Wails-bound: called from the frontend task scheduler panel before adding tasks.
func (a *App) CheckTaskSchedulerOrchestratorReady(sessionKey string) (TaskSchedulerOrchestratorReadiness, error) {
	sessionName, hasSession, err := a.resolveOptionalSessionScopedRequest(sessionKey)
	if err != nil {
		return TaskSchedulerOrchestratorReadiness{}, err
	}
	if !hasSession {
		return TaskSchedulerOrchestratorReadiness{}, nil
	}

	snapshot, err := a.sessionService.FindSessionSnapshotByName(sessionName)
	if err != nil {
		slog.Warn("[DEBUG-TASK-SCHEDULER] readiness: find session snapshot", "session", sessionName, "error", err)
		return TaskSchedulerOrchestratorReadiness{}, fmt.Errorf("find task scheduler session snapshot: %w", err)
	}

	rootPath, err := orchestrator.ResolveSourceRootPath(snapshot)
	if err != nil {
		slog.Warn("[DEBUG-TASK-SCHEDULER] readiness: resolve source root path", "error", err)
		return TaskSchedulerOrchestratorReadiness{}, fmt.Errorf("resolve task scheduler source root: %w", err)
	}

	dbPath := filepath.Join(rootPath, ".myT-x", "orchestrator.db")
	readiness, err := taskscheduler.CheckOrchestratorReady(dbPath)
	if err != nil {
		slog.Warn("[DEBUG-TASK-SCHEDULER] readiness: check orchestrator ready", "dbPath", dbPath, "error", err)
		return TaskSchedulerOrchestratorReadiness{}, fmt.Errorf("check task scheduler orchestrator readiness: %w", err)
	}

	// Check if the active session has any panes.
	hasPanes := false
	for _, w := range snapshot.Windows {
		if len(w.Panes) > 0 {
			hasPanes = true
			break
		}
	}

	return TaskSchedulerOrchestratorReadiness{
		Ready:      readiness.DBExists && readiness.AgentCount > 0 && hasPanes,
		DBExists:   readiness.DBExists,
		AgentCount: readiness.AgentCount,
		HasPanes:   hasPanes,
	}, nil
}
