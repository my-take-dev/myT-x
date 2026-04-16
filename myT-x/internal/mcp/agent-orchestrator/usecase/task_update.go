package usecase

import (
	"context"
	"fmt"
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

const (
	TaskProgressPctMin     = 0
	TaskProgressPctMax     = 100
	MaxTaskProgressNoteLen = 500
)

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

func validateTaskProgressCmd(cmd UpdateTaskProgressCmd) error {
	if cmd.ProgressPct == nil && cmd.ProgressNote == nil {
		return validationError("progress_pct or progress_note is required")
	}
	if cmd.ProgressPct != nil && (*cmd.ProgressPct < TaskProgressPctMin || *cmd.ProgressPct > TaskProgressPctMax) {
		return validationError("progress_pct must be between 0 and 100")
	}
	if cmd.ProgressNote != nil && len([]rune(*cmd.ProgressNote)) > MaxTaskProgressNoteLen {
		return validationError(fmt.Sprintf("progress_note must be %d characters or fewer", MaxTaskProgressNoteLen))
	}
	return nil
}

// AcknowledgeTask records that the assignee has received a task.
func (s *TaskUpdateService) AcknowledgeTask(ctx context.Context, cmd AcknowledgeTaskCmd) (AcknowledgeTaskResult, error) {
	caller, err := preflightAssigneeTaskAgentCaller(ctx, s.resolver, s.agents, s.tasks, s.logger, cmd.AgentName)
	if err != nil {
		return AcknowledgeTaskResult{}, err
	}

	task, err := s.tasks.GetTask(ctx, cmd.TaskID)
	if err != nil {
		return AcknowledgeTaskResult{}, operationError(s.logger, "task is not available", err)
	}
	_, allowed := authorizeAssigneeCaller(task, caller)
	if !allowed {
		return AcknowledgeTaskResult{}, accessDeniedError("caller is not the task assignee")
	}
	if task.Status != domain.TaskStatusPending {
		return AcknowledgeTaskResult{}, conflictError("task is not pending")
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
	if err := validateTaskProgressCmd(cmd); err != nil {
		return UpdateTaskProgressResult{}, err
	}
	caller, err := preflightAssigneeTaskCaller(ctx, s.resolver, s.agents, s.tasks, s.logger)
	if err != nil {
		return UpdateTaskProgressResult{}, err
	}

	task, err := s.tasks.GetTask(ctx, cmd.TaskID)
	if err != nil {
		return UpdateTaskProgressResult{}, operationError(s.logger, "task is not available", err)
	}
	_, allowed := authorizeAssigneeCaller(task, caller)
	if !allowed {
		return UpdateTaskProgressResult{}, accessDeniedError("caller is not the task assignee")
	}
	if task.Status != domain.TaskStatusPending {
		return UpdateTaskProgressResult{}, conflictError("task is not pending")
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
