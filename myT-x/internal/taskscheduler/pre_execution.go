package taskscheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

const (
	preExecProgressResetting        = "resetting"
	preExecProgressWaitingReset     = "waiting_reset"
	preExecProgressSendingReminders = "sending_reminders"
	preExecProgressWaitingIdle      = "waiting_idle"
	preExecProgressIdleTimeout      = "idle_timeout"
	preExecIdlePollInterval         = 2 * time.Second

	// preExecInterPaneResetDelay is the wait time between sending /new to
	// consecutive panes. Without this delay, rapid sequential ConPTY writes
	// can cause input loss on middle panes (observed with 5-pane setups).
	preExecInterPaneResetDelay = 2 * time.Second

	// preExecInterPaneReminderDelay is the wait time between sending role
	// reminder messages to consecutive panes. Matches the orchestrator's
	// bootstrapInterMessageDelay pattern to avoid ConPTY contention.
	preExecInterPaneReminderDelay = 500 * time.Millisecond
)

type idleWaitResult uint8

const (
	idleWaitReady idleWaitResult = iota
	idleWaitTimedOut
	idleWaitCanceled
)

func buildRoleReminderMessage(isTeam bool) string {
	// English is intentional here because the reminder targets agent panes and
	// references MCP/tool names that already use English identifiers.
	message := "Your session was restarted.\n" +
		"Use the orchestrator MCP to confirm your role:\n" +
		"1. Check your pane ID from the $TMUX_PANE environment variable.\n" +
		"2. Run list_agents to confirm your registered profile, role, and skill set.\n"
	if isTeam {
		message += "You will operate as part of the orchestrated team.\n" +
			"Wait for the next task message.\n"
	}
	return message
}

func (s *Service) preExecTargetPanes(items []QueueItem, config QueueConfig) ([]string, error) {
	switch config.PreExecTargetMode {
	case PreExecTargetModeAllPanes:
		sessionName := s.sessionNameSnapshot()
		paneIDs, err := s.deps.GetSessionPaneIDs(sessionName)
		if err != nil {
			return nil, err
		}
		paneIDs = uniqueNonEmptyStrings(paneIDs)
		if len(paneIDs) == 0 {
			return nil, fmt.Errorf("no panes found in session %s", sessionName)
		}
		return paneIDs, nil
	case PreExecTargetModeTaskPanes:
		paneIDs := make([]string, 0, len(items))
		for _, item := range items {
			if item.TargetPaneID == "" {
				continue
			}
			paneIDs = append(paneIDs, item.TargetPaneID)
		}
		paneIDs = uniqueNonEmptyStrings(paneIDs)
		if len(paneIDs) == 0 {
			return nil, fmt.Errorf("no target panes available for pre-execution")
		}
		return paneIDs, nil
	default:
		return nil, fmt.Errorf("unsupported pre-exec target mode: %q", config.PreExecTargetMode)
	}
}

func (s *Service) waitForAllPanesIdle(ctx context.Context, paneIDs []string, timeout time.Duration) idleWaitResult {
	if len(paneIDs) == 0 {
		return idleWaitReady
	}
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	if allPanesQuiet(paneIDs, s.deps.IsPaneQuiet) {
		return idleWaitReady
	}

	ticker := time.NewTicker(preExecIdlePollInterval)
	defer ticker.Stop()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return idleWaitCanceled
		case <-timer.C:
			sessionName := s.sessionNameSnapshot()
			slog.Debug("[DEBUG-TASK-SCHEDULER] pre-execution idle wait timed out",
				"session", sessionName,
				"timeout", timeout,
				"paneCount", len(paneIDs))
			return idleWaitTimedOut
		case <-ticker.C:
			if allPanesQuiet(paneIDs, s.deps.IsPaneQuiet) {
				return idleWaitReady
			}
		}
	}
}

func (s *Service) runPreExecutionPhase(ctx context.Context, generationID string, items []QueueItem, config QueueConfig) bool {
	paneIDs, err := s.preExecTargetPanes(items, config)
	if err != nil {
		s.failPreExecution(generationID, fmt.Sprintf("resolve pre-execution panes: %v", err))
		return false
	}

	s.setPreExecProgress(generationID, preExecProgressResetting)
	resetSuccessCount := 0
	for i, paneID := range paneIDs {
		if err := s.deps.SendClearCommand(paneID, defaultClearCommand); err != nil {
			sessionName := s.sessionNameSnapshot()
			slog.Warn("[WARN-TASK-SCHEDULER] pre-execution reset failed",
				"session", sessionName,
				"paneID", paneID,
				"error", err)
			continue
		}
		resetSuccessCount++
		// Stagger reset commands to prevent ConPTY input loss on middle panes.
		if i < len(paneIDs)-1 {
			if !waitForDuration(ctx, preExecInterPaneResetDelay) {
				return false
			}
		}
	}
	if resetSuccessCount == 0 {
		s.failPreExecution(generationID, "pre-execution reset failed for all target panes")
		return false
	}

	s.setPreExecProgress(generationID, preExecProgressWaitingReset)
	resetDelay := time.Duration(config.PreExecResetDelay) * time.Second
	if !waitForDuration(ctx, resetDelay) {
		return false
	}

	s.setPreExecProgress(generationID, preExecProgressSendingReminders)
	sessionName := s.sessionNameSnapshot()
	reminder := buildRoleReminderMessage(s.deps.IsAgentTeamSession(sessionName))
	reminderSuccessCount := 0
	for i, paneID := range paneIDs {
		// Stagger reminders to prevent ConPTY contention across panes.
		if i > 0 {
			if !waitForDuration(ctx, preExecInterPaneReminderDelay) {
				return false
			}
		}
		if err := s.deps.SendMessagePaste(paneID, reminder); err != nil {
			slog.Warn("[WARN-TASK-SCHEDULER] pre-execution reminder failed",
				"session", sessionName,
				"paneID", paneID,
				"error", err)
			continue
		}
		reminderSuccessCount++
	}
	if reminderSuccessCount == 0 {
		s.failPreExecution(generationID, "pre-execution reminders failed for all target panes")
		return false
	}

	s.setPreExecProgress(generationID, preExecProgressWaitingIdle)
	switch s.waitForAllPanesIdle(ctx, paneIDs, time.Duration(config.PreExecIdleTimeout)*time.Second) {
	case idleWaitCanceled:
		return false
	case idleWaitTimedOut:
		s.setPreExecProgress(generationID, preExecProgressIdleTimeout)
	default:
		s.setPreExecProgress(generationID, "")
	}

	return true
}

func (s *Service) failPreExecution(generationID string, reason string) {
	eventSessionName := ""
	eventGenerationID := ""
	s.mu.Lock()
	if !s.isCurrentGenerationLocked(generationID) {
		s.mu.Unlock()
		return
	}
	s.runStatus = QueueIdle
	s.currentIndex = -1
	s.preExecProgress = ""
	eventSessionName = s.sessionName
	eventGenerationID = s.generationID
	s.mu.Unlock()

	s.emitStopped(reason, eventSessionName, eventGenerationID)
	s.emitUpdated()
}

func (s *Service) setPreExecProgress(generationID string, progress string) {
	s.mu.Lock()
	if s.runStatus != QueuePreparing || !s.isCurrentGenerationLocked(generationID) {
		s.mu.Unlock()
		return
	}
	s.preExecProgress = progress
	s.mu.Unlock()
	s.emitUpdated()
}

func allPanesQuiet(paneIDs []string, isQuiet func(paneID string) bool) bool {
	for _, paneID := range paneIDs {
		if !isQuiet(paneID) {
			return false
		}
	}
	return true
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func waitForDuration(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
