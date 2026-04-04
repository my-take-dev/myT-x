package usecase

import (
	"context"
	"errors"
	"log"
	"time"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

// AcknowledgeTaskCmd marks a task as acknowledged by its assignee.
type AcknowledgeTaskCmd struct {
	AgentName string
	TaskID    string
}

// AcknowledgeTaskResult is the public acknowledgment payload.
type AcknowledgeTaskResult struct {
	TaskID         string
	AgentName      string
	AcknowledgedAt string
}

// UpdateTaskProgressCmd reports progress for an assigned task.
type UpdateTaskProgressCmd struct {
	TaskID       string
	ProgressPct  *int
	ProgressNote *string
}

// UpdateTaskProgressResult is the public progress payload.
type UpdateTaskProgressResult struct {
	TaskID            string
	ProgressPct       *int
	ProgressUpdatedAt string
}

// TaskUpdateService owns assignee-side task state changes.
type TaskUpdateService struct {
	agents   domain.AgentRepository
	tasks    domain.TaskRepository
	resolver domain.SelfPaneResolver
	logger   *log.Logger
}

// NewTaskUpdateService builds a TaskUpdateService.
func NewTaskUpdateService(
	agents domain.AgentRepository,
	tasks domain.TaskRepository,
	resolver domain.SelfPaneResolver,
	logger *log.Logger,
) *TaskUpdateService {
	return &TaskUpdateService{
		agents:   agents,
		tasks:    tasks,
		resolver: resolver,
		logger:   ensureLogger(logger),
	}
}

// AcknowledgeTask records that the assignee has received a task.
func (s *TaskUpdateService) AcknowledgeTask(ctx context.Context, cmd AcknowledgeTaskCmd) (AcknowledgeTaskResult, error) {
	caller, err := resolveCaller(ctx, s.resolver, s.agents, s.logger)
	if err != nil {
		return AcknowledgeTaskResult{}, err
	}
	if !IsTrustedCaller(caller) && caller.Name != cmd.AgentName {
		return AcknowledgeTaskResult{}, errors.New("access denied")
	}
	if err := expirePendingTasks(ctx, s.tasks, s.logger); err != nil {
		return AcknowledgeTaskResult{}, err
	}

	task, err := s.tasks.GetTask(ctx, cmd.TaskID)
	if err != nil {
		return AcknowledgeTaskResult{}, operationError(s.logger, "task is not available", err)
	}
	_, allowed := authorizeResponseCaller(task, caller)
	if !allowed || task.AgentName != cmd.AgentName {
		return AcknowledgeTaskResult{}, errors.New("access denied")
	}
	if task.Status != domain.TaskStatusPending {
		return AcknowledgeTaskResult{}, errors.New("task is not pending")
	}

	acknowledgedAt := task.AcknowledgedAt
	if acknowledgedAt == "" {
		acknowledgedAt = time.Now().UTC().Format(time.RFC3339)
		if err := s.tasks.AcknowledgeTask(ctx, cmd.TaskID, acknowledgedAt); err != nil {
			return AcknowledgeTaskResult{}, operationError(s.logger, "failed to acknowledge task", err)
		}
	}

	return AcknowledgeTaskResult{
		TaskID:         cmd.TaskID,
		AgentName:      cmd.AgentName,
		AcknowledgedAt: acknowledgedAt,
	}, nil
}

// UpdateTaskProgress records structured progress for a pending task.
func (s *TaskUpdateService) UpdateTaskProgress(ctx context.Context, cmd UpdateTaskProgressCmd) (UpdateTaskProgressResult, error) {
	caller, err := resolveCaller(ctx, s.resolver, s.agents, s.logger)
	if err != nil {
		return UpdateTaskProgressResult{}, err
	}
	if cmd.ProgressPct == nil && cmd.ProgressNote == nil {
		return UpdateTaskProgressResult{}, errors.New("progress_pct or progress_note is required")
	}
	if err := expirePendingTasks(ctx, s.tasks, s.logger); err != nil {
		return UpdateTaskProgressResult{}, err
	}

	task, err := s.tasks.GetTask(ctx, cmd.TaskID)
	if err != nil {
		return UpdateTaskProgressResult{}, operationError(s.logger, "task is not available", err)
	}
	_, allowed := authorizeResponseCaller(task, caller)
	if !allowed {
		return UpdateTaskProgressResult{}, errors.New("access denied")
	}
	if task.Status != domain.TaskStatusPending {
		return UpdateTaskProgressResult{}, errors.New("task is not pending")
	}

	progressUpdatedAt := time.Now().UTC().Format(time.RFC3339)
	if err := s.tasks.UpdateTaskProgress(ctx, cmd.TaskID, cmd.ProgressPct, cmd.ProgressNote, progressUpdatedAt); err != nil {
		return UpdateTaskProgressResult{}, operationError(s.logger, "failed to update task progress", err)
	}

	progressPct := cmd.ProgressPct
	if progressPct == nil {
		progressPct = task.ProgressPct
	}

	return UpdateTaskProgressResult{
		TaskID:            cmd.TaskID,
		ProgressPct:       progressPct,
		ProgressUpdatedAt: progressUpdatedAt,
	}, nil
}
