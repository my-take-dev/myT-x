package usecase

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"time"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

// GetMyTasksCmd は自タスク取得コマンド。
type GetMyTasksCmd struct {
	AgentName    string
	StatusFilter domain.TaskStatusFilter
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
	TaskID         string
	FromAgent      string
	SendMessageID  string
	Content        string
	ContentPreview string
	StorageMode    domain.MessageStorageMode
	ArtifactPaths  []string
	PartCount      int
	ContentChars   int
	SHA256         string
	SentAt         string
}

// TaskEntry はタスクエントリ。
type TaskEntry struct {
	TaskID        string
	FromAgent     string
	SenderPaneID  string
	SendMessageID string
	Status        domain.TaskStatus
	SentAt        string
	CompletedAt   string
	IsNowSession  bool
}

// ListAllTasksCmd は全タスク一覧取得コマンド。
type ListAllTasksCmd struct {
	StatusFilter domain.TaskStatusFilter
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
	Status        domain.TaskStatus
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
	Status        domain.TaskStatus
	SentAt        string
	CompletedAt   string
	IsNowSession  bool
	Message       MessageEntry
}

// MessageEntry はメッセージの内容を表す。
type MessageEntry struct {
	Content        string
	CreatedAt      string
	ContentPreview string
	StorageMode    domain.MessageStorageMode
	ArtifactPaths  []string
	PartCount      int
	ContentChars   int
	SHA256         string
}

// TaskResponseEntry captures a stored response for a completed task.
type TaskResponseEntry struct {
	Content        string
	CreatedAt      string
	ContentPreview string
	StorageMode    domain.MessageStorageMode
	ArtifactPaths  []string
	PartCount      int
	ContentChars   int
	SHA256         string
}

// GetTaskDetailCmd resolves rich task detail for a single task.
type GetTaskDetailCmd struct {
	TaskID string
}

// GetTaskDetailResult is the targeted response for a single task.
type GetTaskDetailResult struct {
	TaskID            string
	Status            domain.TaskStatus
	AgentName         string
	GroupID           string
	GroupLabel        string
	CompletedAt       string
	AcknowledgedAt    string
	CancelledAt       string
	CancelReason      string
	ProgressPct       *int
	ProgressNote      string
	ProgressUpdatedAt string
	ExpiresAt         string
	DependsOn         []string
	Message           *MessageEntry
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
	Activated      []ReadyTaskEntry
	StillBlocked   int
	DeliveryFailed int
}

// TaskQueryService はタスクの照会を管理する。
type TaskQueryService struct {
	agents   domain.AgentRepository
	tasks    domain.TaskRepository
	messages domain.MessageRepository
	sender   domain.PanePasteSender
	resolver domain.SelfPaneResolver
	projectRoot string
	logger   *log.Logger
}

// NewTaskQueryService は TaskQueryService を構築する。
func NewTaskQueryService(
	agents domain.AgentRepository,
	tasks domain.TaskRepository,
	messages domain.MessageRepository,
	sender domain.PanePasteSender,
	resolver domain.SelfPaneResolver,
	logger *log.Logger,
	projectRoots ...string,
) *TaskQueryService {
	projectRoot := ""
	if len(projectRoots) > 0 {
		projectRoot = projectRoots[0]
	}
	return &TaskQueryService{
		agents:   agents,
		tasks:    tasks,
		messages: messages,
		sender:   sender,
		resolver: resolver,
		projectRoot: projectRoot,
		logger:   ensureLogger(logger),
	}
}

const DefaultMaxInline = 3

// GetMyTasks は自分宛のタスクを取得する。
// Pending unacknowledged tasks include inline message content and are
// auto-acknowledged best-effort when returned inline.
func (s *TaskQueryService) GetMyTasks(ctx context.Context, cmd GetMyTasksCmd) (GetMyTasksResult, error) {
	_, err := preflightAssigneeTaskAgentCaller(ctx, s.resolver, s.agents, s.tasks, s.logger, cmd.AgentName)
	if err != nil {
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
					TaskID:         t.ID,
					FromAgent:      t.SenderName,
					SendMessageID:  t.SendMessageID,
					Content:        msg.Content,
					ContentPreview: msg.ContentPreview,
					StorageMode:    msg.StorageMode,
					ArtifactPaths:  domain.ResolveArtifactPaths(s.projectRoot, msg.ArtifactPaths),
					PartCount:      msg.PartCount,
					ContentChars:   msg.ContentChars,
					SHA256:         msg.SHA256,
					SentAt:         t.SentAt,
				})
				if msg.StorageMode == "" || msg.StorageMode == domain.MessageStorageInline {
					acknowledgedAt := time.Now().UTC().Format(time.RFC3339)
					if ackErr := s.tasks.AcknowledgeTask(ctx, t.ID, acknowledgedAt); ackErr != nil {
						logf(s.logger, "get_my_tasks: auto acknowledge task %s: %v", t.ID, ackErr)
					}
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
	caller, err := preflightAssigneeTaskAgentCaller(ctx, s.resolver, s.agents, s.tasks, s.logger, cmd.AgentName)
	if err != nil {
		return GetTaskMessageResult{}, err
	}

	task, err := s.tasks.GetTaskBySendMessageID(ctx, cmd.SendMessageID)
	if err != nil {
		return GetTaskMessageResult{}, operationError(s.logger, "task not found", err)
	}

	if _, allowed := authorizeAssigneeCaller(task, caller); !allowed {
		return GetTaskMessageResult{}, accessDeniedError("caller is not the task assignee")
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
			Content:        msg.Content,
			CreatedAt:      msg.CreatedAt,
			ContentPreview: msg.ContentPreview,
			StorageMode:    msg.StorageMode,
			ArtifactPaths:  domain.ResolveArtifactPaths(s.projectRoot, msg.ArtifactPaths),
			PartCount:      msg.PartCount,
			ContentChars:   msg.ContentChars,
			SHA256:         msg.SHA256,
		},
	}, nil
}

// ListAllTasks は全タスクの状態一覧を取得する。
func (s *TaskQueryService) ListAllTasks(ctx context.Context, cmd ListAllTasksCmd) (ListAllTasksResult, error) {
	if _, err := preflightTaskCaller(ctx, s.resolver, s.agents, s.tasks, s.logger); err != nil {
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
	caller, err := preflightTaskCaller(ctx, s.resolver, s.agents, s.tasks, s.logger)
	if err != nil {
		return GetTaskDetailResult{}, err
	}

	task, err := s.tasks.GetTask(ctx, cmd.TaskID)
	if err != nil {
		return GetTaskDetailResult{}, operationError(s.logger, "task is not available", err)
	}
	if !authorizeTaskDetailCaller(task, caller) {
		return GetTaskDetailResult{}, accessDeniedError("caller is not allowed to inspect this task")
	}

	result := GetTaskDetailResult{
		TaskID:            task.ID,
		Status:            task.Status,
		AgentName:         task.AgentName,
		GroupID:           task.GroupID,
		CompletedAt:       task.CompletedAt,
		AcknowledgedAt:    task.AcknowledgedAt,
		CancelledAt:       task.CancelledAt,
		CancelReason:      task.CancelReason,
		ProgressPct:       task.ProgressPct,
		ProgressNote:      task.ProgressNote,
		ProgressUpdatedAt: task.ProgressUpdatedAt,
		ExpiresAt:         task.ExpiresAt,
	}
	if task.SendMessageID != "" {
		message, err := s.messages.GetMessage(ctx, task.SendMessageID)
		if err != nil {
			return GetTaskDetailResult{}, operationError(s.logger, "message is not available", err)
		}
		result.Message = &MessageEntry{
			Content:        message.Content,
			CreatedAt:      message.CreatedAt,
			ContentPreview: message.ContentPreview,
			StorageMode:    message.StorageMode,
			ArtifactPaths:  domain.ResolveArtifactPaths(s.projectRoot, message.ArtifactPaths),
			PartCount:      message.PartCount,
			ContentChars:   message.ContentChars,
			SHA256:         message.SHA256,
		}
	}
	if task.GroupID != "" {
		group, err := s.tasks.GetTaskGroup(ctx, task.GroupID)
		if err != nil {
			if !errors.Is(err, domain.ErrNotFound) {
				return GetTaskDetailResult{}, operationError(s.logger, "task group is not available", err)
			}
		} else {
			result.GroupLabel = group.Label
		}
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
			Content:        response.Content,
			CreatedAt:      response.CreatedAt,
			ContentPreview: response.ContentPreview,
			StorageMode:    response.StorageMode,
			ArtifactPaths:  domain.ResolveArtifactPaths(s.projectRoot, response.ArtifactPaths),
			PartCount:      response.PartCount,
			ContentChars:   response.ContentChars,
			SHA256:         response.SHA256,
		}
	}

	return result, nil
}

// ActivateReadyTasks activates blocked tasks whose dependencies are fully completed.
func (s *TaskQueryService) ActivateReadyTasks(ctx context.Context, cmd ActivateReadyTasksCmd) (ActivateReadyTasksResult, error) {
	if _, err := preflightTaskCaller(ctx, s.resolver, s.agents, s.tasks, s.logger); err != nil {
		return ActivateReadyTasksResult{}, err
	}
	activated, stillBlocked, err := s.tasks.ActivateReadyTasks(ctx, time.Now().UTC().Format(time.RFC3339), cmd.AgentName)
	if err != nil {
		return ActivateReadyTasksResult{}, operationError(s.logger, "failed to activate ready tasks", err)
	}
	delivered, deliveryFailed := s.deliverActivatedTasks(ctx, activated)
	entries := make([]ReadyTaskEntry, 0, len(delivered))
	for _, task := range delivered {
		entries = append(entries, ReadyTaskEntry{
			TaskID:    task.ID,
			AgentName: task.AgentName,
		})
	}
	return ActivateReadyTasksResult{
		Activated:      entries,
		StillBlocked:   stillBlocked,
		DeliveryFailed: deliveryFailed,
	}, nil
}

func (s *TaskQueryService) deliverActivatedTasks(ctx context.Context, tasks []domain.Task) ([]domain.Task, int) {
	delivered := make([]domain.Task, 0, len(tasks))
	deliveryFailed := 0
	for _, task := range tasks {
		deliveredTask, failed := s.deliverActivatedTask(ctx, task)
		if deliveredTask {
			delivered = append(delivered, task)
		}
		if failed {
			deliveryFailed++
		}
	}
	return delivered, deliveryFailed
}

func (s *TaskQueryService) deliverActivatedTask(ctx context.Context, task domain.Task) (bool, bool) {
	if s.sender == nil || s.messages == nil {
		return true, false
	}
	if task.SendMessageID == "" || task.AssigneePaneID == "" || domain.IsVirtualPaneID(task.AssigneePaneID) {
		return true, false
	}

	msg, err := s.messages.GetMessage(ctx, task.SendMessageID)
	if err != nil {
		if failErr := s.tasks.MarkTaskFailed(ctx, task.ID); failErr != nil {
			slog.Error("[ERROR-MCP-ORCH] activate_ready_tasks message lookup failed and mark failed could not persist",
				"taskID", task.ID,
				"messageID", task.SendMessageID,
				"lookupErr", err,
				"markFailedErr", failErr,
			)
			return false, true
		}
		slog.Warn("[WARN-MCP-ORCH] activate_ready_tasks message lookup failed",
			"taskID", task.ID,
			"messageID", task.SendMessageID,
			"error", err,
		)
		return false, true
	}
	if err := s.sender.SendKeysPaste(ctx, task.AssigneePaneID, deliveryTextForStoredMessage(s.projectRoot, task.ID, msg)); err != nil {
		if failErr := s.tasks.MarkTaskFailed(ctx, task.ID); failErr != nil {
			slog.Error("[ERROR-MCP-ORCH] activate_ready_tasks delivery failed and mark failed could not persist",
				"taskID", task.ID,
				"paneID", task.AssigneePaneID,
				"sendErr", err,
				"markFailedErr", failErr,
			)
			return false, true
		}
		slog.Warn("[WARN-MCP-ORCH] activate_ready_tasks delivery failed",
			"taskID", task.ID,
			"paneID", task.AssigneePaneID,
			"error", err,
		)
		return false, true
	}
	return true, false
}

func authorizeTaskDetailCaller(task domain.Task, caller domain.Agent) bool {
	if authorizeTaskSenderCaller(task, caller, "") {
		return true
	}
	_, allowed := authorizeResponseCaller(task, caller)
	return allowed
}
