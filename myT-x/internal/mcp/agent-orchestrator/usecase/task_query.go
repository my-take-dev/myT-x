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
	TaskID        string
	SenderPaneID  string
	SendMessageID string
	Status        string
	SentAt        string
	CompletedAt   string
	IsNowSession  bool
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
	TaskID        string
	AgentName     string
	SenderPaneID  string
	SendMessageID string
	Status        string
	SentAt        string
	CompletedAt   string
	IsNowSession  bool
}

// GetMyTaskCmd は単一タスク取得コマンド。
type GetMyTaskCmd struct {
	AgentName     string
	SendMessageID string
}

// GetMyTaskResult は単一タスク取得結果。
type GetMyTaskResult struct {
	TaskID        string
	AgentName     string
	SenderPaneID  string
	SendMessageID string
	Status        string
	SentAt        string
	CompletedAt   string
	IsNowSession  bool
	Message       MessageEntry
}

// MessageEntry はメッセージの内容を表す。
type MessageEntry struct {
	Content   string
	CreatedAt string
}

// TaskQueryService はタスクの照会を管理する。
type TaskQueryService struct {
	agents   domain.AgentRepository
	tasks    domain.TaskRepository
	messages domain.MessageRepository
	resolver domain.SelfPaneResolver
	logger   *log.Logger
}

// NewTaskQueryService は TaskQueryService を構築する。
func NewTaskQueryService(
	agents domain.AgentRepository,
	tasks domain.TaskRepository,
	messages domain.MessageRepository,
	resolver domain.SelfPaneResolver,
	logger *log.Logger,
) *TaskQueryService {
	return &TaskQueryService{
		agents:   agents,
		tasks:    tasks,
		messages: messages,
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
			TaskID:        t.ID,
			SenderPaneID:  t.SenderPaneID,
			SendMessageID: t.SendMessageID,
			Status:        t.Status,
			SentAt:        t.SentAt,
			CompletedAt:   t.CompletedAt,
			IsNowSession:  t.IsNowSession,
		})
	}

	return GetMyTasksResult{
		AgentName:            cmd.AgentName,
		Tasks:                entries,
		ResponseInstructions: buildResponseInstruction(""),
	}, nil
}

// GetMyTask は send_message_id から自分宛の単一タスクとメッセージ本文を取得する。
func (s *TaskQueryService) GetMyTask(ctx context.Context, cmd GetMyTaskCmd) (GetMyTaskResult, error) {
	caller, err := resolveCaller(ctx, s.resolver, s.agents, s.logger)
	if err != nil {
		return GetMyTaskResult{}, err
	}

	if !IsTrustedCaller(caller) && caller.Name != cmd.AgentName {
		return GetMyTaskResult{}, errors.New("access denied")
	}

	task, err := s.tasks.GetTaskBySendMessageID(ctx, cmd.SendMessageID)
	if err != nil {
		return GetMyTaskResult{}, operationError(s.logger, "task not found", err)
	}

	if task.AgentName != cmd.AgentName {
		return GetMyTaskResult{}, errors.New("access denied")
	}

	msg, err := s.messages.GetMessage(ctx, cmd.SendMessageID)
	if err != nil {
		return GetMyTaskResult{}, operationError(s.logger, "message not found", err)
	}

	return GetMyTaskResult{
		TaskID:        task.ID,
		AgentName:     task.AgentName,
		SenderPaneID:  task.SenderPaneID,
		SendMessageID: task.SendMessageID,
		Status:        task.Status,
		SentAt:        task.SentAt,
		CompletedAt:   task.CompletedAt,
		IsNowSession:  task.IsNowSession,
		Message: MessageEntry{
			Content:   msg.Content,
			CreatedAt: msg.CreatedAt,
		},
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
			TaskID:        t.ID,
			AgentName:     t.AgentName,
			SenderPaneID:  t.SenderPaneID,
			SendMessageID: t.SendMessageID,
			Status:        t.Status,
			SentAt:        t.SentAt,
			CompletedAt:   t.CompletedAt,
			IsNowSession:  t.IsNowSession,
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
