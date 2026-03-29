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
	SenderInstanceID            string
}

// SendTaskResult はタスク送信結果。
type SendTaskResult struct {
	TaskID       string
	AgentName    string
	PaneID       string
	SenderPaneID string
	SentAt       string
}

// TaskDispatchService はタスク送信を管理する。
type TaskDispatchService struct {
	agents   domain.AgentRepository
	tasks    domain.TaskRepository
	messages domain.MessageRepository
	sender   domain.PaneSender
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
	logger *log.Logger,
) *TaskDispatchService {
	return &TaskDispatchService{
		agents:   agents,
		tasks:    tasks,
		messages: messages,
		sender:   sender,
		logger:   ensureLogger(logger),
		randRead: rand.Read,
	}
}

// Send はタスクを送信する。
func (s *TaskDispatchService) Send(ctx context.Context, cmd SendTaskCmd) (SendTaskResult, error) {
	senderAgent, err := s.agents.GetAgent(ctx, cmd.FromAgent)
	if err != nil {
		return SendTaskResult{}, operationError(s.logger, "sender agent is not available", err)
	}

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

	now := time.Now().UTC().Format(time.RFC3339)

	// メッセージを保存
	msgID, err := generateIDWith(s.randRead, "m-", "generate message id")
	if err != nil {
		return SendTaskResult{}, operationError(s.logger, "failed to generate message id", err)
	}
	if err := s.messages.SaveMessage(ctx, domain.TaskMessage{
		ID:        msgID,
		Content:   cmd.Message,
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
	}
	if err := s.tasks.CreateTask(ctx, task); err != nil {
		return SendTaskResult{}, operationError(s.logger, "failed to persist task", err)
	}

	if err := s.sender.SendKeys(ctx, agent.PaneID, sendMessage); err != nil {
		if failErr := s.tasks.MarkTaskFailed(ctx, taskID); failErr != nil {
			logf(s.logger, "mark task %s failed: %v", taskID, failErr)
			return SendTaskResult{}, operationError(s.logger, "message delivery failed; task may remain pending", fmt.Errorf("%w (mark task failed: %v)", err, failErr))
		}
		return SendTaskResult{}, operationError(s.logger, "message delivery failed", err)
	}

	result := SendTaskResult{
		TaskID:       taskID,
		AgentName:    cmd.AgentName,
		PaneID:       agent.PaneID,
		SenderPaneID: senderAgent.PaneID,
		SentAt:       now,
	}

	return result, nil
}

func buildResponseInstruction(taskID string) string {
	if taskID == "" {
		taskID = "<task_id>"
	}
	return "応答方法：send_response MCPツールで返信してください（タスク完了も同時記録されます）。" +
		"\ntask_id=" + taskID +
		"\nsend_response(task_id=\"" + taskID + "\", message=\"...\") を行いましょう。task_id が抜けるとタスクを完了できません。" +
		"\n他エージェントへの相談・依頼：tmux のペインTitleからエージェントを確認するか、" +
		"SQLiteのagentsテーブルから相手を検索し、send_task MCPツールで送信してください。"
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
