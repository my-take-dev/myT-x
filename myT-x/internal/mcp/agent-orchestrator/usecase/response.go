package usecase

import (
	"context"
	"errors"
	"log"
	"time"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

// SendResponseCmd は応答送信コマンド。
type SendResponseCmd struct {
	Message string
	TaskID  string
}

// SendResponseResult は応答送信結果。
type SendResponseResult struct {
	SentTo      string
	SentToName  string
	TaskID      string
	TaskStatus  string
	CompletedAt string
	Warning     string
}

// ResponseService はタスクへの応答を管理する。
type ResponseService struct {
	agents   domain.AgentRepository
	tasks    domain.TaskRepository
	messages domain.MessageRepository
	sender   domain.PaneSender
	resolver domain.SelfPaneResolver
	logger   *log.Logger
}

// NewResponseService は ResponseService を構築する。
func NewResponseService(
	agents domain.AgentRepository,
	tasks domain.TaskRepository,
	messages domain.MessageRepository,
	sender domain.PaneSender,
	resolver domain.SelfPaneResolver,
	logger *log.Logger,
) *ResponseService {
	return &ResponseService{
		agents:   agents,
		tasks:    tasks,
		messages: messages,
		sender:   sender,
		resolver: resolver,
		logger:   ensureLogger(logger),
	}
}

// Send はタスク送信者に応答を返す。
func (s *ResponseService) Send(ctx context.Context, cmd SendResponseCmd) (SendResponseResult, error) {
	logf(s.logger, "send_response start task_id=%s message_length=%d", cmd.TaskID, len([]rune(cmd.Message)))
	caller, err := resolveCaller(ctx, s.resolver, s.agents, s.logger)
	if err != nil {
		return SendResponseResult{}, err
	}

	task, err := s.tasks.GetTask(ctx, cmd.TaskID)
	if err != nil {
		return SendResponseResult{}, operationError(s.logger, "task is not available", err)
	}
	authMode, allowed := authorizeResponseCaller(task, caller)
	logf(
		s.logger,
		"send_response task loaded task_id=%s task_agent=%s task_assignee_pane=%s sender=%s status=%s caller=%s caller_pane=%s auth_mode=%s",
		task.ID,
		task.AgentName,
		task.AssigneePaneID,
		task.SenderName,
		task.Status,
		caller.Name,
		caller.PaneID,
		authMode,
	)
	if !allowed {
		logf(
			s.logger,
			"send_response access denied task_id=%s caller=%s caller_pane=%s owner=%s assignee_pane=%s auth_mode=%s",
			task.ID,
			caller.Name,
			caller.PaneID,
			task.AgentName,
			task.AssigneePaneID,
			authMode,
		)
		return SendResponseResult{}, errors.New("access denied")
	}
	if task.Status != domain.TaskStatusPending {
		logf(s.logger, "send_response rejected non-pending task_id=%s status=%s", task.ID, task.Status)
		return SendResponseResult{}, errors.New("task is not pending")
	}

	targetName := task.SenderName
	if targetName == "" {
		logf(s.logger, "send_response missing sender task_id=%s", task.ID)
		return SendResponseResult{}, errors.New("task sender is unknown; cannot deliver response")
	}
	target, err := s.agents.GetAgent(ctx, targetName)
	if err != nil {
		return SendResponseResult{}, operationError(s.logger, "response target is not available", err)
	}
	logf(s.logger, "send_response target resolved task_id=%s target=%s pane_id=%s", task.ID, target.Name, target.PaneID)

	if err := s.sender.SendKeys(ctx, target.PaneID, cmd.Message); err != nil {
		return SendResponseResult{}, operationError(s.logger, "response delivery failed", err)
	}
	logf(s.logger, "send_response delivered task_id=%s target=%s pane_id=%s", task.ID, target.Name, target.PaneID)

	result := SendResponseResult{
		SentTo:     target.PaneID,
		SentToName: target.Name,
	}

	// レスポンスを保存
	now := time.Now().UTC().Format(time.RFC3339)

	// TaskID is always set: the caller needs it regardless of downstream failures.
	result.TaskID = cmd.TaskID

	respID, err := generateResponseID()
	if err != nil {
		result.Warning = "message delivered but response id generation failed"
		logf(s.logger, "generate response id for task %s: %v", cmd.TaskID, err)
		completeTaskBestEffort(s, ctx, cmd.TaskID, "", now, &result, "response id failure")
		return result, nil
	}
	if err := s.messages.SaveResponse(ctx, domain.TaskMessage{
		ID:        respID,
		Content:   cmd.Message,
		CreatedAt: now,
	}); err != nil {
		result.Warning = "message delivered but response persistence failed"
		logf(s.logger, "save response for task %s: %v", cmd.TaskID, err)
		completeTaskBestEffort(s, ctx, cmd.TaskID, "", now, &result, "save response failure")
		return result, nil
	}

	if err := s.tasks.CompleteTask(ctx, cmd.TaskID, respID, now); err != nil {
		result.Warning = "message delivered but task completion update failed"
		result.TaskStatus = "unknown"
		logf(s.logger, "complete task %s after response: %v", cmd.TaskID, err)
	} else {
		logf(s.logger, "send_response completed task_id=%s completed_at=%s", cmd.TaskID, now)
		result.TaskStatus = domain.TaskStatusCompleted
		result.CompletedAt = now
	}

	return result, nil
}

// completeTaskBestEffort attempts to mark a task as completed after a non-fatal
// error (e.g. response ID generation or persistence failure). On double-fault
// (CompleteTask also fails), the task status is set to "unknown" and a warning
// is logged indicating both the original and completion failures.
func completeTaskBestEffort(s *ResponseService, ctx context.Context, taskID, respID, now string, result *SendResponseResult, context string) {
	if completeErr := s.tasks.CompleteTask(ctx, taskID, respID, now); completeErr != nil {
		logf(s.logger, "double-fault: complete task %s after %s also failed: %v", taskID, context, completeErr)
		result.TaskStatus = "unknown"
	} else {
		result.TaskStatus = domain.TaskStatusCompleted
		result.CompletedAt = now
	}
}

func authorizeResponseCaller(task domain.Task, caller domain.Agent) (string, bool) {
	if IsTrustedCaller(caller) {
		return "trusted", true
	}
	if task.AssigneePaneID != "" && caller.PaneID == task.AssigneePaneID {
		return "pane", true
	}
	if task.AgentName == caller.Name {
		return "agent_name", true
	}
	return "denied", false
}
