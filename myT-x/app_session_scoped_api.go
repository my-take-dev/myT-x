package main

import (
	"errors"
	"fmt"
	"strings"

	"myT-x/internal/singletaskrunner"
	"myT-x/internal/taskscheduler"
)

var errSessionKeyRequired = errors.New("session key is required")

func buildSessionKey(sessionName string, sessionID int) string {
	return fmt.Sprintf("%s:%d", sessionName, sessionID)
}

func (a *App) requireExistingSessionKey(sessionKey string) (string, error) {
	trimmedKey := strings.TrimSpace(sessionKey)
	if trimmedKey == "" {
		return "", errSessionKeyRequired
	}

	sessions, err := a.requireSessions()
	if err != nil {
		return "", err
	}

	for _, snapshot := range sessions.Snapshot() {
		if buildSessionKey(snapshot.Name, snapshot.ID) == trimmedKey {
			return snapshot.Name, nil
		}
	}

	return "", fmt.Errorf("session key not found: %s", trimmedKey)
}

func (a *App) activeSessionIdentity(sessionName string) (string, string, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return "", "", errors.New("no active session")
	}

	snapshot, err := a.sessionService.FindSessionSnapshotByName(sessionName)
	if err != nil {
		return "", "", fmt.Errorf("find active session snapshot: %w", err)
	}

	return snapshot.Name, buildSessionKey(snapshot.Name, snapshot.ID), nil
}

func (a *App) requireActiveSessionKey(sessionKey string) (string, error) {
	trimmedKey := strings.TrimSpace(sessionKey)
	activeSessionName := strings.TrimSpace(a.sessionService.GetActiveSessionName())
	if trimmedKey == "" {
		if activeSessionName == "" {
			return "", errors.New("no active session")
		}
		return "", errSessionKeyRequired
	}

	sessionName, activeKey, err := a.activeSessionIdentity(activeSessionName)
	if err != nil {
		return "", err
	}
	if trimmedKey != activeKey {
		return "", fmt.Errorf("session key mismatch: requested %s, active %s", trimmedKey, activeKey)
	}
	return sessionName, nil
}

func (a *App) resolveOptionalSessionScopedRequest(sessionKey string) (string, bool, error) {
	if strings.TrimSpace(sessionKey) == "" {
		if strings.TrimSpace(a.sessionService.GetActiveSessionName()) == "" {
			return "", false, nil
		}
		return "", false, errSessionKeyRequired
	}

	sessionName, err := a.requireActiveSessionKey(sessionKey)
	if err != nil {
		return "", false, err
	}
	return sessionName, true, nil
}

func defaultSingleTaskRunnerQueueStatus() SingleTaskRunnerQueueStatus {
	return SingleTaskRunnerQueueStatus{
		Items:          []SingleTaskRunnerQueueItem{},
		RunStatus:      singletaskrunner.QueueIdle,
		CurrentIndex:   -1,
		SessionName:    "",
		GenerationID:   "",
		ClearDelaySec:  singletaskrunner.DefaultClearDelay,
		LastStopReason: "",
	}
}

func defaultTaskSchedulerQueueStatus() TaskSchedulerQueueStatus {
	return TaskSchedulerQueueStatus{
		Config: taskscheduler.QueueConfig{
			PreExecResetDelay:  defaultPreExecResetDelay,
			PreExecIdleTimeout: defaultPreExecIdleTimeout,
			PreExecTargetMode:  taskscheduler.PreExecTargetMode(defaultPreExecTargetMode),
		},
		Items:        []TaskSchedulerQueueItem{},
		RunStatus:    taskscheduler.QueueIdle,
		CurrentIndex: -1,
		SessionName:  "",
		GenerationID: "",
	}
}
