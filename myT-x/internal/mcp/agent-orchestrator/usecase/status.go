package usecase

import (
	"context"
	"errors"
	"log"
	"time"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

// UpdateStatusCmd updates the caller's reported work status.
type UpdateStatusCmd struct {
	AgentName     string
	Status        string
	CurrentTaskID string
	Note          string
}

// UpdateStatusResult is the public response for update_status.
type UpdateStatusResult struct {
	AgentName        string
	Status           string
	UpdatedAt        string
	RedeliveredCount int
}

// GetAgentStatusCmd reads a single agent status.
type GetAgentStatusCmd struct {
	AgentName string
}

// GetAgentStatusResult is the targeted status response.
type GetAgentStatusResult struct {
	AgentName          string
	Status             string
	CurrentTaskID      string
	Note               string
	SecondsSinceUpdate *int64
}

// StatusService manages agent activity state.
type StatusService struct {
	agents   domain.AgentRepository
	statuses domain.AgentStatusRepository
	tasks    domain.TaskRepository
	messages domain.MessageRepository
	sender   domain.PaneSender
	resolver domain.SelfPaneResolver
	logger   *log.Logger
}

// NewStatusService builds a StatusService.
func NewStatusService(
	agents domain.AgentRepository,
	statuses domain.AgentStatusRepository,
	tasks domain.TaskRepository,
	messages domain.MessageRepository,
	sender domain.PaneSender,
	resolver domain.SelfPaneResolver,
	logger *log.Logger,
) *StatusService {
	return &StatusService{
		agents:   agents,
		statuses: statuses,
		tasks:    tasks,
		messages: messages,
		sender:   sender,
		resolver: resolver,
		logger:   ensureLogger(logger),
	}
}

// UpdateStatus records the latest work state for an agent.
func (s *StatusService) UpdateStatus(ctx context.Context, cmd UpdateStatusCmd) (UpdateStatusResult, error) {
	caller, err := resolveCaller(ctx, s.resolver, s.agents, s.logger)
	if err != nil {
		return UpdateStatusResult{}, err
	}
	if !IsTrustedCaller(caller) && caller.Name != cmd.AgentName {
		return UpdateStatusResult{}, errors.New("access denied")
	}
	if !isValidAgentWorkStatus(cmd.Status) {
		return UpdateStatusResult{}, errors.New("invalid agent status")
	}
	if _, err := s.agents.GetAgent(ctx, cmd.AgentName); err != nil {
		return UpdateStatusResult{}, operationError(s.logger, "agent is not available", err)
	}

	updatedAt := time.Now().UTC().Format(time.RFC3339)
	if err := s.statuses.UpsertAgentStatus(ctx, domain.AgentStatus{
		AgentName:     cmd.AgentName,
		Status:        cmd.Status,
		CurrentTaskID: cmd.CurrentTaskID,
		Note:          cmd.Note,
		UpdatedAt:     updatedAt,
	}); err != nil {
		return UpdateStatusResult{}, operationError(s.logger, "failed to update agent status", err)
	}

	result := UpdateStatusResult{
		AgentName: cmd.AgentName,
		Status:    cmd.Status,
		UpdatedAt: updatedAt,
	}

	// idle 報告時に未 acknowledge の pending タスクを再配信
	if cmd.Status == domain.AgentWorkStatusIdle {
		result.RedeliveredCount = s.redeliverPendingTasks(ctx, cmd.AgentName)
	}

	return result, nil
}

// GetAgentStatus returns the latest reported work state for an agent.
func (s *StatusService) GetAgentStatus(ctx context.Context, cmd GetAgentStatusCmd) (GetAgentStatusResult, error) {
	if _, err := resolveCaller(ctx, s.resolver, s.agents, s.logger); err != nil {
		return GetAgentStatusResult{}, err
	}
	if _, err := s.agents.GetAgent(ctx, cmd.AgentName); err != nil {
		return GetAgentStatusResult{}, operationError(s.logger, "agent is not available", err)
	}

	status, err := s.statuses.GetAgentStatus(ctx, cmd.AgentName)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return GetAgentStatusResult{
				AgentName: cmd.AgentName,
				Status:    domain.AgentWorkStatusUnknown,
			}, nil
		}
		return GetAgentStatusResult{}, operationError(s.logger, "failed to load agent status", err)
	}

	result := GetAgentStatusResult{
		AgentName:     status.AgentName,
		Status:        status.Status,
		CurrentTaskID: status.CurrentTaskID,
		Note:          status.Note,
	}
	if status.UpdatedAt != "" {
		updatedAt, parseErr := time.Parse(time.RFC3339, status.UpdatedAt)
		if parseErr == nil {
			seconds := max(int64(time.Since(updatedAt).Seconds()), 0)
			result.SecondsSinceUpdate = &seconds
		}
	}

	return result, nil
}

// redeliverPendingTasks re-sends unacknowledged pending tasks to the agent's pane via SendKeys.
// Successfully re-delivered tasks are auto-acknowledged to avoid repeated delivery loops.
// Returns the number of tasks successfully re-delivered.
func (s *StatusService) redeliverPendingTasks(ctx context.Context, agentName string) int {
	if s.tasks == nil || s.messages == nil || s.sender == nil {
		return 0
	}

	agent, err := s.agents.GetAgent(ctx, agentName)
	if err != nil {
		logf(s.logger, "update_status redeliver: get agent %s: %v", agentName, err)
		return 0
	}
	if domain.IsVirtualPaneID(agent.PaneID) {
		return 0
	}

	tasks, err := s.tasks.ListTasks(ctx, domain.TaskFilter{
		Status:    domain.TaskStatusPending,
		AgentName: agentName,
	})
	if err != nil {
		logf(s.logger, "update_status redeliver: list tasks for %s: %v", agentName, err)
		return 0
	}

	count := 0
	for _, task := range tasks {
		if task.AcknowledgedAt != "" || task.SendMessageID == "" {
			continue
		}
		msg, msgErr := s.messages.GetMessage(ctx, task.SendMessageID)
		if msgErr != nil {
			logf(s.logger, "update_status redeliver: get message %s for task %s: %v", task.SendMessageID, task.ID, msgErr)
			continue
		}
		if sendErr := s.sender.SendKeys(ctx, agent.PaneID, msg.Content); sendErr != nil {
			logf(s.logger, "update_status redeliver: send task %s to pane %s: %v", task.ID, agent.PaneID, sendErr)
			continue
		}
		acknowledgedAt := time.Now().UTC().Format(time.RFC3339)
		if ackErr := s.tasks.AcknowledgeTask(ctx, task.ID, acknowledgedAt); ackErr != nil {
			logf(s.logger, "update_status redeliver: acknowledge task %s after pane delivery: %v", task.ID, ackErr)
		}
		logf(s.logger, "update_status redeliver: re-sent task %s to %s (pane %s)", task.ID, agentName, agent.PaneID)
		count++
	}
	return count
}

func isValidAgentWorkStatus(status string) bool {
	switch status {
	case domain.AgentWorkStatusIdle, domain.AgentWorkStatusBusy, domain.AgentWorkStatusWorking:
		return true
	default:
		return false
	}
}
