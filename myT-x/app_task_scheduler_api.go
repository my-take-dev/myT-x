package main

import (
	"errors"

	"myT-x/internal/taskscheduler"
)

// TaskSchedulerQueueStatus is the frontend-safe representation of the queue status.
type TaskSchedulerQueueStatus = taskscheduler.QueueStatus

// TaskSchedulerQueueItem is the frontend-safe representation of a queue item.
type TaskSchedulerQueueItem = taskscheduler.QueueItem

// TaskSchedulerQueueConfig is the frontend-safe representation of queue config.
type TaskSchedulerQueueConfig = taskscheduler.QueueConfig

// taskSchedulerForActiveSession returns the Service for the active session.
func (a *App) taskSchedulerForActiveSession() (*taskscheduler.Service, error) {
	session := a.sessionService.GetActiveSessionName()
	if session == "" {
		return nil, errors.New("no active session")
	}
	return a.taskSchedulerManager.GetOrCreate(session), nil
}

// GetTaskSchedulerStatus returns the current queue status.
// Wails-bound: called from the frontend task scheduler panel.
func (a *App) GetTaskSchedulerStatus() TaskSchedulerQueueStatus {
	session := a.sessionService.GetActiveSessionName()
	if session == "" {
		return TaskSchedulerQueueStatus{
			Items:        []TaskSchedulerQueueItem{},
			RunStatus:    "idle",
			CurrentIndex: -1,
		}
	}
	return a.taskSchedulerManager.GetStatus(session)
}

// StartTaskScheduler begins executing the task queue.
// Wails-bound: called from the frontend task scheduler panel.
func (a *App) StartTaskScheduler(config TaskSchedulerQueueConfig, items []TaskSchedulerQueueItem) error {
	svc, err := a.taskSchedulerForActiveSession()
	if err != nil {
		return err
	}
	return svc.Start(config, items)
}

// StopTaskScheduler stops the queue and marks remaining items as skipped.
// Wails-bound: called from the frontend task scheduler panel.
func (a *App) StopTaskScheduler() error {
	svc, err := a.taskSchedulerForActiveSession()
	if err != nil {
		return err
	}
	return svc.Stop()
}

// PauseTaskScheduler pauses the queue by cancelling the current worker.
// In-progress items are preserved and resumed when ResumeTaskScheduler is called.
// Wails-bound: called from the frontend task scheduler panel.
func (a *App) PauseTaskScheduler() error {
	svc, err := a.taskSchedulerForActiveSession()
	if err != nil {
		return err
	}
	return svc.Pause()
}

// ResumeTaskScheduler resumes a paused queue.
// Wails-bound: called from the frontend task scheduler panel.
func (a *App) ResumeTaskScheduler() error {
	svc, err := a.taskSchedulerForActiveSession()
	if err != nil {
		return err
	}
	return svc.Resume()
}

// AddTaskSchedulerItem adds a new task to the end of the queue.
// Wails-bound: called from the frontend task scheduler panel.
func (a *App) AddTaskSchedulerItem(title, message, targetPaneID string, clearBefore bool, clearCommand string) error {
	svc, err := a.taskSchedulerForActiveSession()
	if err != nil {
		return err
	}
	return svc.AddItem(title, message, targetPaneID, clearBefore, clearCommand)
}

// RemoveTaskSchedulerItem removes a pending item from the queue.
// Wails-bound: called from the frontend task scheduler panel.
func (a *App) RemoveTaskSchedulerItem(id string) error {
	svc, err := a.taskSchedulerForActiveSession()
	if err != nil {
		return err
	}
	return svc.RemoveItem(id)
}

// ReorderTaskSchedulerItems reorders items by their IDs.
// Wails-bound: called from the frontend task scheduler panel.
func (a *App) ReorderTaskSchedulerItems(orderedIDs []string) error {
	svc, err := a.taskSchedulerForActiveSession()
	if err != nil {
		return err
	}
	return svc.ReorderItems(orderedIDs)
}

// UpdateTaskSchedulerItem updates a pending item's fields.
// Wails-bound: called from the frontend task scheduler panel.
func (a *App) UpdateTaskSchedulerItem(id, title, message, targetPaneID string, clearBefore bool, clearCommand string) error {
	svc, err := a.taskSchedulerForActiveSession()
	if err != nil {
		return err
	}
	return svc.UpdateItem(id, title, message, targetPaneID, clearBefore, clearCommand)
}
