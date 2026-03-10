package usecase

import (
	"context"
	"errors"
	"log"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

// GetMyTasksCmd は自タスク取得コマンド。
type GetMyTasksCmd struct {
	AgentName    string
	StatusFilter string
}

// GetMyTasksResult は自タスク取得結果。
type GetMyTasksResult struct {
	AgentName            string
	Tasks                []TaskEntry
	ResponseInstructions string
}

// TaskEntry はタスクエントリ。
type TaskEntry struct {
	TaskID      string
	Label       string
	Status      string
	SentAt      string
	CompletedAt string
}

// CheckTasksCmd はタスク確認コマンド。
type CheckTasksCmd struct {
	StatusFilter string
	AgentName    string
}

// CheckTasksResult はタスク確認結果。
type CheckTasksResult struct {
	Tasks     []CheckTaskEntry
	Pending   int
	Completed int
	Failed    int
	Abandoned int
}

// CheckTaskEntry はタスク確認のエントリ。
type CheckTaskEntry struct {
	TaskID      string
	AgentName   string
	Label       string
	Status      string
	SentAt      string
	CompletedAt string
	Notes       string
}

// TaskQueryService はタスクの照会を管理する。
type TaskQueryService struct {
	agents   domain.AgentRepository
	tasks    domain.TaskRepository
	resolver domain.SelfPaneResolver
	logger   *log.Logger
}

// NewTaskQueryService は TaskQueryService を構築する。
func NewTaskQueryService(
	agents domain.AgentRepository,
	tasks domain.TaskRepository,
	resolver domain.SelfPaneResolver,
	logger *log.Logger,
) *TaskQueryService {
	return &TaskQueryService{
		agents:   agents,
		tasks:    tasks,
		resolver: resolver,
		logger:   ensureLogger(logger),
	}
}

// GetMyTasks は自分宛のタスクを取得する。
func (s *TaskQueryService) GetMyTasks(ctx context.Context, cmd GetMyTasksCmd) (GetMyTasksResult, error) {
	caller, err := resolveCaller(ctx, s.resolver, s.agents, s.logger)
	if err != nil {
		return GetMyTasksResult{}, err
	}

	if !IsTrustedCaller(caller) && caller.Name != cmd.AgentName {
		return GetMyTasksResult{}, errors.New("access denied")
	}

	tasks, err := s.tasks.ListTasks(ctx, domain.TaskFilter{
		Status:    cmd.StatusFilter,
		AgentName: cmd.AgentName,
	})
	if err != nil {
		return GetMyTasksResult{}, operationError(s.logger, "failed to list tasks", err)
	}

	entries := make([]TaskEntry, 0, len(tasks))
	for _, t := range tasks {
		entries = append(entries, TaskEntry{
			TaskID:      t.ID,
			Label:       t.Label,
			Status:      t.Status,
			SentAt:      t.SentAt,
			CompletedAt: t.CompletedAt,
		})
	}

	return GetMyTasksResult{
		AgentName:            cmd.AgentName,
		Tasks:                entries,
		ResponseInstructions: buildResponseInstruction(""),
	}, nil
}

// CheckTasks はタスク状態を確認する。
func (s *TaskQueryService) CheckTasks(ctx context.Context, cmd CheckTasksCmd) (CheckTasksResult, error) {
	if _, err := resolveCaller(ctx, s.resolver, s.agents, s.logger); err != nil {
		return CheckTasksResult{}, err
	}

	tasks, err := s.tasks.ListTasks(ctx, domain.TaskFilter{
		Status:    cmd.StatusFilter,
		AgentName: cmd.AgentName,
	})
	if err != nil {
		return CheckTasksResult{}, operationError(s.logger, "failed to list tasks", err)
	}

	entries := make([]CheckTaskEntry, 0, len(tasks))
	pending, completed, failed, abandoned := 0, 0, 0, 0
	for _, t := range tasks {
		entries = append(entries, CheckTaskEntry{
			TaskID:      t.ID,
			AgentName:   t.AgentName,
			Label:       t.Label,
			Status:      t.Status,
			SentAt:      t.SentAt,
			CompletedAt: t.CompletedAt,
			Notes:       t.Notes,
		})
		switch t.Status {
		case "pending":
			pending++
		case "completed":
			completed++
		case "failed":
			failed++
		case "abandoned":
			abandoned++
		}
	}

	return CheckTasksResult{
		Tasks:     entries,
		Pending:   pending,
		Completed: completed,
		Failed:    failed,
		Abandoned: abandoned,
	}, nil
}
