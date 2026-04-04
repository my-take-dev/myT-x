package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

// SendTaskCmd はタスク送信コマンド。
type SendTaskCmd struct {
	AgentName                   string
	FromAgent                   string
	Message                     string
	IncludeResponseInstructions bool
	ExpiresAfterMinutes         int
	DependsOn                   []string
	GroupID                     string
	SenderInstanceID            string
}

// SendTaskBatchItemCmd describes one task within a grouped dispatch.
type SendTaskBatchItemCmd struct {
	AgentName                   string
	Message                     string
	IncludeResponseInstructions bool
	ExpiresAfterMinutes         int
}

// SendTasksCmd sends multiple tasks under one batch identifier.
type SendTasksCmd struct {
	FromAgent        string
	Tasks            []SendTaskBatchItemCmd
	GroupLabel       string
	SenderInstanceID string
}

// SendTaskResult はタスク送信結果。
type SendTaskResult struct {
	TaskID       string
	AgentName    string
	PaneID       string
	SenderPaneID string
	SentAt       string
}

// SendTasksItemResult captures the per-target outcome of a batch dispatch.
type SendTasksItemResult struct {
	TaskID    string
	AgentName string
	Error     string
}

// SendTasksSummary is the aggregate outcome for a batch dispatch.
type SendTasksSummary struct {
	Sent   int
	Failed int
}

// SendTasksResult is the public result for grouped task dispatch.
type SendTasksResult struct {
	GroupID string
	Results []SendTasksItemResult
	Summary SendTasksSummary
}

// CancelTaskCmd cancels a pending task created by the caller.
type CancelTaskCmd struct {
	TaskID string
	Reason string
}

// CancelTaskResult is the public result for task cancellation.
type CancelTaskResult struct {
	TaskID string
	Status string
}

// TaskDispatchService はタスク送信を管理する。
type TaskDispatchService struct {
	agents   domain.AgentRepository
	tasks    domain.TaskRepository
	messages domain.MessageRepository
	sender   domain.PaneSender
	resolver domain.SelfPaneResolver
	logger   *log.Logger
	// randRead is the random byte source for ID generation.
	// Defaults to crypto/rand.Read. Tests inject deterministic sources.
	randRead func([]byte) (int, error)
}

// NewTaskDispatchService は TaskDispatchService を構築する。
func NewTaskDispatchService(
	agents domain.AgentRepository,
	tasks domain.TaskRepository,
	messages domain.MessageRepository,
	sender domain.PaneSender,
	resolver domain.SelfPaneResolver,
	logger *log.Logger,
) *TaskDispatchService {
	return &TaskDispatchService{
		agents:   agents,
		tasks:    tasks,
		messages: messages,
		sender:   sender,
		resolver: resolver,
		logger:   ensureLogger(logger),
		randRead: rand.Read,
	}
}

// Send intentionally remains available to any caller as long as from_agent
// resolves to a registered sender. The product spec allows direct task
// handoffs without requiring the caller pane itself to be registered.
func (s *TaskDispatchService) Send(ctx context.Context, cmd SendTaskCmd) (SendTaskResult, error) {
	senderAgent, err := s.agents.GetAgent(ctx, cmd.FromAgent)
	if err != nil {
		return SendTaskResult{}, operationError(s.logger, "sender agent is not available", err)
	}
	return s.sendWithSender(ctx, senderAgent, cmd)
}

// SendBatch intentionally requires a registered caller because bulk fan-out is
// restricted to in-session agents even though single send_task calls are not.
func (s *TaskDispatchService) SendBatch(ctx context.Context, cmd SendTasksCmd) (SendTasksResult, error) {
	if _, err := resolveCaller(ctx, s.resolver, s.agents, s.logger); err != nil {
		return SendTasksResult{}, err
	}
	senderAgent, err := s.agents.GetAgent(ctx, cmd.FromAgent)
	if err != nil {
		return SendTasksResult{}, operationError(s.logger, "sender agent is not available", err)
	}
	groupID, err := generateIDWith(s.randRead, "g-", "generate group id")
	if err != nil {
		return SendTasksResult{}, operationError(s.logger, "failed to generate group id", err)
	}
	createdAt := time.Now().UTC().Format(time.RFC3339)
	if err := s.tasks.CreateTaskGroup(ctx, domain.TaskGroup{
		ID:        groupID,
		Label:     cmd.GroupLabel,
		CreatedAt: createdAt,
	}); err != nil {
		return SendTasksResult{}, operationError(s.logger, "failed to persist task group", err)
	}

	results := make([]SendTasksItemResult, 0, len(cmd.Tasks))
	summary := SendTasksSummary{}
	for _, item := range cmd.Tasks {
		result, err := s.sendWithSender(ctx, senderAgent, SendTaskCmd{
			AgentName:                   item.AgentName,
			FromAgent:                   cmd.FromAgent,
			Message:                     item.Message,
			IncludeResponseInstructions: item.IncludeResponseInstructions,
			ExpiresAfterMinutes:         item.ExpiresAfterMinutes,
			GroupID:                     groupID,
			SenderInstanceID:            cmd.SenderInstanceID,
		})
		if err != nil {
			logf(s.logger, "send_tasks: task for %s failed: %v", item.AgentName, err)
			summary.Failed++
			results = append(results, SendTasksItemResult{
				AgentName: item.AgentName,
				Error:     err.Error(),
			})
			continue
		}
		summary.Sent++
		results = append(results, SendTasksItemResult{
			TaskID:    result.TaskID,
			AgentName: result.AgentName,
		})
	}
	if summary.Sent == 0 {
		if err := s.tasks.DeleteTaskGroup(ctx, groupID); err != nil {
			return SendTasksResult{}, operationError(s.logger, "failed to clean up empty task group", err)
		}
		groupID = ""
	}

	return SendTasksResult{
		GroupID: groupID,
		Results: results,
		Summary: summary,
	}, nil
}

func (s *TaskDispatchService) sendWithSender(ctx context.Context, senderAgent domain.Agent, cmd SendTaskCmd) (SendTaskResult, error) {
	agent, err := s.agents.GetAgent(ctx, cmd.AgentName)
	if err != nil {
		return SendTaskResult{}, operationError(s.logger, "target agent is not available", err)
	}

	if domain.IsVirtualPaneID(agent.PaneID) {
		return SendTaskResult{}, errors.New("cannot send task to virtual pane agent")
	}

	taskID, err := generateIDWith(s.randRead, "t-", "generate task id")
	if err != nil {
		return SendTaskResult{}, operationError(s.logger, "failed to generate task id", err)
	}

	sendMessage := cmd.Message
	if cmd.IncludeResponseInstructions && !strings.Contains(cmd.Message, "応答方法：send_response MCPツール") {
		sendMessage = cmd.Message + "\n\n---\n" + buildResponseInstruction(taskID)
	}

	nowTime := time.Now().UTC()
	now := nowTime.Format(time.RFC3339)
	expiresAt := ""
	if cmd.ExpiresAfterMinutes > 0 {
		expiresAt = nowTime.Add(time.Duration(cmd.ExpiresAfterMinutes) * time.Minute).Format(time.RFC3339)
	}

	// メッセージを保存
	msgID, err := generateIDWith(s.randRead, "m-", "generate message id")
	if err != nil {
		return SendTaskResult{}, operationError(s.logger, "failed to generate message id", err)
	}
	if err := s.messages.SaveMessage(ctx, domain.TaskMessage{
		ID:        msgID,
		Content:   sendMessage,
		CreatedAt: now,
	}); err != nil {
		return SendTaskResult{}, operationError(s.logger, "failed to persist message", err)
	}

	task := domain.Task{
		ID:               taskID,
		AgentName:        cmd.AgentName,
		AssigneePaneID:   agent.PaneID,
		SenderPaneID:     senderAgent.PaneID,
		SenderName:       cmd.FromAgent,
		SenderInstanceID: cmd.SenderInstanceID,
		SendMessageID:    msgID,
		Status:           domain.TaskStatusPending,
		SentAt:           now,
		ExpiresAt:        expiresAt,
		GroupID:          cmd.GroupID,
	}
	if len(cmd.DependsOn) > 0 {
		task.Status = domain.TaskStatusBlocked
	}

	if err := s.tasks.CreateTaskWithDependencies(ctx, task, cmd.DependsOn); err != nil {
		return SendTaskResult{}, operationError(s.logger, "failed to persist task", err)
	}

	result := SendTaskResult{
		TaskID:       taskID,
		AgentName:    cmd.AgentName,
		PaneID:       agent.PaneID,
		SenderPaneID: senderAgent.PaneID,
		SentAt:       now,
	}

	if task.Status == domain.TaskStatusBlocked {
		return result, nil
	}

	if err := s.sender.SendKeys(ctx, agent.PaneID, sendMessage); err != nil {
		if failErr := s.tasks.MarkTaskFailed(ctx, taskID); failErr != nil {
			logf(s.logger, "mark task %s failed: %v", taskID, failErr)
			return SendTaskResult{}, operationError(s.logger, "message delivery failed; task may remain pending", fmt.Errorf("%w (mark task failed: %v)", err, failErr))
		}
		return SendTaskResult{}, operationError(s.logger, "message delivery failed", err)
	}

	return result, nil
}

// CancelTask cancels a pending task when requested by its sender.
func (s *TaskDispatchService) CancelTask(ctx context.Context, cmd CancelTaskCmd) (CancelTaskResult, error) {
	caller, err := resolveCaller(ctx, s.resolver, s.agents, s.logger)
	if err != nil {
		return CancelTaskResult{}, err
	}

	task, err := s.tasks.GetTask(ctx, cmd.TaskID)
	if err != nil {
		return CancelTaskResult{}, operationError(s.logger, "task is not available", err)
	}
	if !authorizeTaskSenderCaller(task, caller) {
		return CancelTaskResult{}, errors.New("access denied")
	}

	cancelledAt := time.Now().UTC().Format(time.RFC3339)
	if err := s.tasks.CancelTask(ctx, cmd.TaskID, cancelledAt, cmd.Reason); err != nil {
		return CancelTaskResult{}, operationError(s.logger, "failed to cancel task", err)
	}

	return CancelTaskResult{
		TaskID: cmd.TaskID,
		Status: domain.TaskStatusCancelled,
	}, nil
}

func authorizeTaskSenderCaller(task domain.Task, caller domain.Agent) bool {
	if IsTrustedCaller(caller) {
		return true
	}
	if task.SenderName != "" && caller.Name == task.SenderName {
		return true
	}
	if task.SenderPaneID != "" && caller.PaneID == task.SenderPaneID {
		return true
	}
	return false
}

func buildResponseInstruction(taskID string) string {
	if taskID == "" {
		taskID = "<task_id>"
	}
	return "応答方法：send_response MCPツールで返信してください（タスク完了も同時記録されます）。" +
		"\ntask_id=" + taskID +
		"\nsend_response(task_id=\"" + taskID + "\", message=\"...\") を行いましょう。task_id が抜けるとタスクを完了できません。" +
		"\n他エージェントへの相談・依頼：tmux のペインTitleからエージェントを確認するか、" +
		"list_agents MCPツールで相手を確認し、send_task MCPツールで送信してください。"
}

// generateIDWith generates a random hex-encoded ID with the given prefix.
// The readFn parameter allows tests to inject deterministic random sources.
// Output format: prefix + hex(6 bytes) = prefix + 12 hex characters.
func generateIDWith(readFn func([]byte) (int, error), prefix, errContext string) (string, error) {
	b := make([]byte, 6)
	if _, err := readFn(b); err != nil {
		return "", fmt.Errorf("%s: %w", errContext, err)
	}
	return prefix + hex.EncodeToString(b), nil
}
