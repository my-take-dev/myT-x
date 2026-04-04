package usecase

import (
	"context"
	"errors"
	"log"
	"time"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

// GetMyTasksCmd は自タスク取得コマンド。
type GetMyTasksCmd struct {
	AgentName    string
	StatusFilter string
	MaxInline    int // インラインメッセージの最大数（デフォルト: DefaultMaxInline）
}

// GetMyTasksResult は自タスク取得結果。
type GetMyTasksResult struct {
	AgentName            string
	Tasks                []TaskEntry
	InlineMessages       []InlineMessageEntry
	ResponseInstructions string
}

// InlineMessageEntry は get_my_tasks でインライン返却されるメッセージ。
type InlineMessageEntry struct {
	TaskID        string
	FromAgent     string
	SendMessageID string
	Content       string
	SentAt        string
}

// TaskEntry はタスクエントリ。
type TaskEntry struct {
	TaskID        string
	FromAgent     string
	SenderPaneID  string
	SendMessageID string
	Status        string
	SentAt        string
	CompletedAt   string
	IsNowSession  bool
}

// ListAllTasksCmd は全タスク一覧取得コマンド。
type ListAllTasksCmd struct {
	StatusFilter string
	AgentName    string
}

// ListAllTasksResult は全タスク一覧取得結果。
type ListAllTasksResult struct {
	Tasks     []ListAllTaskEntry
	Pending   int
	Blocked   int
	Completed int
	Failed    int
	Abandoned int
	Cancelled int
	Expired   int
}

// ListAllTaskEntry は全タスク一覧のエントリ。
type ListAllTaskEntry struct {
	TaskID        string
	AgentName     string
	SenderPaneID  string
	SendMessageID string
	Status        string
	SentAt        string
	CompletedAt   string
	IsNowSession  bool
}

// GetTaskMessageCmd はタスクメッセージ取得コマンド。
type GetTaskMessageCmd struct {
	AgentName     string
	SendMessageID string
}

// GetTaskMessageResult はタスクメッセージ取得結果。
type GetTaskMessageResult struct {
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

// TaskResponseEntry captures a stored response for a completed task.
type TaskResponseEntry struct {
	Content   string
	CreatedAt string
}

// GetTaskDetailCmd resolves rich task detail for a single task.
type GetTaskDetailCmd struct {
	TaskID string
}

// GetTaskDetailResult is the targeted response for a single task.
type GetTaskDetailResult struct {
	TaskID            string
	Status            string
	AgentName         string
	CompletedAt       string
	AcknowledgedAt    string
	CancelledAt       string
	CancelReason      string
	ProgressPct       *int
	ProgressNote      string
	ProgressUpdatedAt string
	ExpiresAt         string
	DependsOn         []string
	Response          *TaskResponseEntry
}

// ActivateReadyTasksCmd requests activation of blocked tasks whose dependencies are satisfied.
type ActivateReadyTasksCmd struct {
	AgentName string
}

// ReadyTaskEntry is one task that transitioned from blocked to pending.
type ReadyTaskEntry struct {
	TaskID    string
	AgentName string
}

// ActivateReadyTasksResult reports newly activated tasks and how many remain blocked.
type ActivateReadyTasksResult struct {
	Activated    []ReadyTaskEntry
	StillBlocked int
}

// TaskQueryService はタスクの照会を管理する。
type TaskQueryService struct {
	agents   domain.AgentRepository
	tasks    domain.TaskRepository
	messages domain.MessageRepository
	sender   domain.PaneSender
	resolver domain.SelfPaneResolver
	logger   *log.Logger
}

// NewTaskQueryService は TaskQueryService を構築する。
func NewTaskQueryService(
	agents domain.AgentRepository,
	tasks domain.TaskRepository,
	messages domain.MessageRepository,
	sender domain.PaneSender,
	resolver domain.SelfPaneResolver,
	logger *log.Logger,
) *TaskQueryService {
	return &TaskQueryService{
		agents:   agents,
		tasks:    tasks,
		messages: messages,
		sender:   sender,
		resolver: resolver,
		logger:   ensureLogger(logger),
	}
}

const DefaultMaxInline = 3

// GetMyTasks は自分宛のタスクを取得する。
// pending かつ未 acknowledge のタスクはメッセージ本文をインラインで返却し、自動 acknowledge する。
func (s *TaskQueryService) GetMyTasks(ctx context.Context, cmd GetMyTasksCmd) (GetMyTasksResult, error) {
	caller, err := resolveCaller(ctx, s.resolver, s.agents, s.logger)
	if err != nil {
		return GetMyTasksResult{}, err
	}

	if !IsTrustedCaller(caller) && caller.Name != cmd.AgentName {
		return GetMyTasksResult{}, errors.New("access denied")
	}
	if err := expirePendingTasks(ctx, s.tasks, s.logger); err != nil {
		return GetMyTasksResult{}, err
	}

	tasks, err := s.tasks.ListTasks(ctx, domain.TaskFilter{
		Status:    cmd.StatusFilter,
		AgentName: cmd.AgentName,
	})
	if err != nil {
		return GetMyTasksResult{}, operationError(s.logger, "failed to list tasks", err)
	}

	maxInline := cmd.MaxInline
	if maxInline <= 0 {
		maxInline = DefaultMaxInline
	}

	entries := make([]TaskEntry, 0, len(tasks))
	var inlineMessages []InlineMessageEntry
	for _, t := range tasks {
		entries = append(entries, TaskEntry{
			TaskID:        t.ID,
			FromAgent:     t.SenderName,
			SenderPaneID:  t.SenderPaneID,
			SendMessageID: t.SendMessageID,
			Status:        t.Status,
			SentAt:        t.SentAt,
			CompletedAt:   t.CompletedAt,
			IsNowSession:  t.IsNowSession,
		})

		if t.Status == domain.TaskStatusPending && t.AcknowledgedAt == "" && t.SendMessageID != "" && len(inlineMessages) < maxInline {
			msg, msgErr := s.messages.GetMessage(ctx, t.SendMessageID)
			if msgErr != nil {
				logf(s.logger, "get_my_tasks: inline message fetch for task %s: %v", t.ID, msgErr)
			} else {
				inlineMessages = append(inlineMessages, InlineMessageEntry{
					TaskID:        t.ID,
					FromAgent:     t.SenderName,
					SendMessageID: t.SendMessageID,
					Content:       msg.Content,
					SentAt:        t.SentAt,
				})
				acknowledgedAt := time.Now().UTC().Format(time.RFC3339)
				if ackErr := s.tasks.AcknowledgeTask(ctx, t.ID, acknowledgedAt); ackErr != nil {
					logf(s.logger, "get_my_tasks: auto-acknowledge task %s: %v", t.ID, ackErr)
				}
			}
		}
	}

	return GetMyTasksResult{
		AgentName:            cmd.AgentName,
		Tasks:                entries,
		InlineMessages:       inlineMessages,
		ResponseInstructions: buildResponseInstruction(""),
	}, nil
}

// GetTaskMessage は send_message_id から自分宛の単一タスクとメッセージ本文を取得する。
func (s *TaskQueryService) GetTaskMessage(ctx context.Context, cmd GetTaskMessageCmd) (GetTaskMessageResult, error) {
	caller, err := resolveCaller(ctx, s.resolver, s.agents, s.logger)
	if err != nil {
		return GetTaskMessageResult{}, err
	}

	if !IsTrustedCaller(caller) && caller.Name != cmd.AgentName {
		return GetTaskMessageResult{}, errors.New("access denied")
	}
	if err := expirePendingTasks(ctx, s.tasks, s.logger); err != nil {
		return GetTaskMessageResult{}, err
	}

	task, err := s.tasks.GetTaskBySendMessageID(ctx, cmd.SendMessageID)
	if err != nil {
		return GetTaskMessageResult{}, operationError(s.logger, "task not found", err)
	}

	if task.AgentName != cmd.AgentName {
		return GetTaskMessageResult{}, errors.New("access denied")
	}

	msg, err := s.messages.GetMessage(ctx, cmd.SendMessageID)
	if err != nil {
		return GetTaskMessageResult{}, operationError(s.logger, "message not found", err)
	}

	return GetTaskMessageResult{
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

// ListAllTasks は全タスクの状態一覧を取得する。
func (s *TaskQueryService) ListAllTasks(ctx context.Context, cmd ListAllTasksCmd) (ListAllTasksResult, error) {
	if _, err := resolveCaller(ctx, s.resolver, s.agents, s.logger); err != nil {
		return ListAllTasksResult{}, err
	}
	if err := expirePendingTasks(ctx, s.tasks, s.logger); err != nil {
		return ListAllTasksResult{}, err
	}

	tasks, err := s.tasks.ListTasks(ctx, domain.TaskFilter{
		Status:    cmd.StatusFilter,
		AgentName: cmd.AgentName,
	})
	if err != nil {
		return ListAllTasksResult{}, operationError(s.logger, "failed to list tasks", err)
	}

	entries := make([]ListAllTaskEntry, 0, len(tasks))
	pending, blocked, completed, failed, abandoned, cancelled, expired := 0, 0, 0, 0, 0, 0, 0
	for _, t := range tasks {
		entries = append(entries, ListAllTaskEntry{
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
		case domain.TaskStatusPending:
			pending++
		case domain.TaskStatusBlocked:
			blocked++
		case domain.TaskStatusCompleted:
			completed++
		case domain.TaskStatusFailed:
			failed++
		case domain.TaskStatusAbandoned:
			abandoned++
		case domain.TaskStatusCancelled:
			cancelled++
		case domain.TaskStatusExpired:
			expired++
		}
	}

	return ListAllTasksResult{
		Tasks:     entries,
		Pending:   pending,
		Blocked:   blocked,
		Completed: completed,
		Failed:    failed,
		Abandoned: abandoned,
		Cancelled: cancelled,
		Expired:   expired,
	}, nil
}

// GetTaskDetail returns targeted metadata for a single task and its response when available.
func (s *TaskQueryService) GetTaskDetail(ctx context.Context, cmd GetTaskDetailCmd) (GetTaskDetailResult, error) {
	caller, err := resolveCaller(ctx, s.resolver, s.agents, s.logger)
	if err != nil {
		return GetTaskDetailResult{}, err
	}
	if err := expirePendingTasks(ctx, s.tasks, s.logger); err != nil {
		return GetTaskDetailResult{}, err
	}

	task, err := s.tasks.GetTask(ctx, cmd.TaskID)
	if err != nil {
		return GetTaskDetailResult{}, operationError(s.logger, "task is not available", err)
	}
	if !authorizeTaskDetailCaller(task, caller) {
		return GetTaskDetailResult{}, errors.New("access denied")
	}

	result := GetTaskDetailResult{
		TaskID:            task.ID,
		Status:            task.Status,
		AgentName:         task.AgentName,
		CompletedAt:       task.CompletedAt,
		AcknowledgedAt:    task.AcknowledgedAt,
		CancelledAt:       task.CancelledAt,
		CancelReason:      task.CancelReason,
		ProgressPct:       task.ProgressPct,
		ProgressNote:      task.ProgressNote,
		ProgressUpdatedAt: task.ProgressUpdatedAt,
		ExpiresAt:         task.ExpiresAt,
	}
	dependsOn, err := s.tasks.GetTaskDependencies(ctx, task.ID)
	if err != nil {
		return GetTaskDetailResult{}, operationError(s.logger, "task dependencies are not available", err)
	}
	result.DependsOn = dependsOn

	if task.Status == domain.TaskStatusCompleted && task.SendResponseID != "" {
		response, err := s.messages.GetResponse(ctx, task.SendResponseID)
		if err != nil {
			return GetTaskDetailResult{}, operationError(s.logger, "response is not available", err)
		}
		result.Response = &TaskResponseEntry{
			Content:   response.Content,
			CreatedAt: response.CreatedAt,
		}
	}

	return result, nil
}

// ActivateReadyTasks activates blocked tasks whose dependencies are fully completed.
func (s *TaskQueryService) ActivateReadyTasks(ctx context.Context, cmd ActivateReadyTasksCmd) (ActivateReadyTasksResult, error) {
	if _, err := resolveCaller(ctx, s.resolver, s.agents, s.logger); err != nil {
		return ActivateReadyTasksResult{}, err
	}
	activated, stillBlocked, err := s.tasks.ActivateReadyTasks(ctx, time.Now().UTC().Format(time.RFC3339), cmd.AgentName)
	if err != nil {
		return ActivateReadyTasksResult{}, operationError(s.logger, "failed to activate ready tasks", err)
	}
	s.deliverActivatedTasks(ctx, activated)
	entries := make([]ReadyTaskEntry, 0, len(activated))
	for _, task := range activated {
		entries = append(entries, ReadyTaskEntry{
			TaskID:    task.ID,
			AgentName: task.AgentName,
		})
	}
	return ActivateReadyTasksResult{
		Activated:    entries,
		StillBlocked: stillBlocked,
	}, nil
}

func (s *TaskQueryService) deliverActivatedTasks(ctx context.Context, tasks []domain.Task) {
	for _, task := range tasks {
		s.deliverActivatedTask(ctx, task)
	}
}

func (s *TaskQueryService) deliverActivatedTask(ctx context.Context, task domain.Task) {
	if s.sender == nil || s.messages == nil {
		return
	}
	if task.SendMessageID == "" || task.AssigneePaneID == "" || domain.IsVirtualPaneID(task.AssigneePaneID) {
		return
	}

	msg, err := s.messages.GetMessage(ctx, task.SendMessageID)
	if err != nil {
		logf(s.logger, "activate_ready_tasks: get message %s for task %s: %v", task.SendMessageID, task.ID, err)
		return
	}
	if err := s.sender.SendKeys(ctx, task.AssigneePaneID, msg.Content); err != nil {
		if failErr := s.tasks.MarkTaskFailed(ctx, task.ID); failErr != nil {
			logf(s.logger, "activate_ready_tasks: deliver task %s to pane %s failed: %v (mark failed: %v)", task.ID, task.AssigneePaneID, err, failErr)
			return
		}
		logf(s.logger, "activate_ready_tasks: deliver task %s to pane %s: %v", task.ID, task.AssigneePaneID, err)
	}
}

func authorizeTaskDetailCaller(task domain.Task, caller domain.Agent) bool {
	if authorizeTaskSenderCaller(task, caller) {
		return true
	}
	_, allowed := authorizeResponseCaller(task, caller)
	return allowed
}
