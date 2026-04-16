package main

import (
	"myT-x/internal/singletaskrunner"
)

// SingleTaskRunnerQueueStatus is the frontend-safe representation of the queue status.
type SingleTaskRunnerQueueStatus = singletaskrunner.QueueStatus

// SingleTaskRunnerQueueItem is the frontend-safe representation of a queue item.
type SingleTaskRunnerQueueItem = singletaskrunner.QueueItem

func (a *App) singleTaskRunnerForActiveSession(sessionKey string) (*singletaskrunner.Service, error) {
	sessionName, err := a.requireActiveSessionKey(sessionKey)
	if err != nil {
		return nil, err
	}
	return a.singleTaskRunnerManager.GetOrCreate(sessionName), nil
}

// GetSingleTaskRunnerStatus returns the current queue status.
func (a *App) GetSingleTaskRunnerStatus(sessionKey string) (SingleTaskRunnerQueueStatus, error) {
	sessionName, hasSession, err := a.resolveOptionalSessionScopedRequest(sessionKey)
	if err != nil {
		return defaultSingleTaskRunnerQueueStatus(), err
	}
	if !hasSession {
		return defaultSingleTaskRunnerQueueStatus(), nil
	}
	return a.singleTaskRunnerManager.GetStatus(sessionName), nil
}

// StartSingleTaskRunner starts the queue for the active session.
func (a *App) StartSingleTaskRunner(sessionKey string) error {
	svc, err := a.singleTaskRunnerForActiveSession(sessionKey)
	if err != nil {
		return err
	}
	return svc.Start()
}

// StopSingleTaskRunner stops the queue for the active session.
func (a *App) StopSingleTaskRunner(sessionKey string) error {
	svc, err := a.singleTaskRunnerForActiveSession(sessionKey)
	if err != nil {
		return err
	}
	return svc.Stop()
}

// AddSingleTaskRunnerItem adds a new item to the queue.
func (a *App) AddSingleTaskRunnerItem(sessionKey, title, message, targetPaneID string, clearBefore bool, clearCommand string) error {
	svc, err := a.singleTaskRunnerForActiveSession(sessionKey)
	if err != nil {
		return err
	}
	return svc.AddItem(title, message, targetPaneID, clearBefore, clearCommand)
}

// RemoveSingleTaskRunnerItem removes an editable item from the queue.
func (a *App) RemoveSingleTaskRunnerItem(sessionKey, id string) error {
	svc, err := a.singleTaskRunnerForActiveSession(sessionKey)
	if err != nil {
		return err
	}
	return svc.RemoveItem(id)
}

// UpdateSingleTaskRunnerItem updates an editable item.
func (a *App) UpdateSingleTaskRunnerItem(sessionKey, id, title, message, targetPaneID string, clearBefore bool, clearCommand string) error {
	svc, err := a.singleTaskRunnerForActiveSession(sessionKey)
	if err != nil {
		return err
	}
	return svc.UpdateItem(id, title, message, targetPaneID, clearBefore, clearCommand)
}

// ReorderSingleTaskRunnerItems reorders queue items by ID.
func (a *App) ReorderSingleTaskRunnerItems(sessionKey string, orderedIDs []string) error {
	svc, err := a.singleTaskRunnerForActiveSession(sessionKey)
	if err != nil {
		return err
	}
	return svc.ReorderItems(orderedIDs)
}

// SetSingleTaskRunnerClearDelay updates the clear-command delay.
func (a *App) SetSingleTaskRunnerClearDelay(sessionKey string, delaySec int) error {
	svc, err := a.singleTaskRunnerForActiveSession(sessionKey)
	if err != nil {
		return err
	}
	return svc.SetClearDelay(delaySec)
}

// GetSingleTaskRunnerClearDelay returns the current clear-command delay.
func (a *App) GetSingleTaskRunnerClearDelay(sessionKey string) (int, error) {
	sessionName, hasSession, err := a.resolveOptionalSessionScopedRequest(sessionKey)
	if err != nil {
		return singletaskrunner.DefaultClearDelay, err
	}
	if !hasSession {
		return singletaskrunner.DefaultClearDelay, nil
	}
	return a.singleTaskRunnerManager.GetClearDelay(sessionName), nil
}
