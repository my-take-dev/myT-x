package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

var randRead = rand.Read

// SendTaskCmd はタスク送信コマンド。
type SendTaskCmd struct {
	AgentName                   string
	FromAgent                   string
	Message                     string
	TaskLabel                   string
	IncludeResponseInstructions bool
}

// SendTaskResult はタスク送信結果。
type SendTaskResult struct {
	TaskID    string
	AgentName string
	PaneID    string
	SentAt    string
}

// TaskDispatchService はタスク送信を管理する。
type TaskDispatchService struct {
	agents domain.AgentRepository
	tasks  domain.TaskRepository
	sender domain.PaneSender
	logger *log.Logger
}

// NewTaskDispatchService は TaskDispatchService を構築する。
func NewTaskDispatchService(
	agents domain.AgentRepository,
	tasks domain.TaskRepository,
	sender domain.PaneSender,
	logger *log.Logger,
) *TaskDispatchService {
	return &TaskDispatchService{
		agents: agents,
		tasks:  tasks,
		sender: sender,
		logger: ensureLogger(logger),
	}
}

// Send はタスクを送信する。
func (s *TaskDispatchService) Send(ctx context.Context, cmd SendTaskCmd) (SendTaskResult, error) {
	if _, err := s.agents.GetAgent(ctx, cmd.FromAgent); err != nil {
		return SendTaskResult{}, operationError(s.logger, "sender agent is not available", err)
	}

	agent, err := s.agents.GetAgent(ctx, cmd.AgentName)
	if err != nil {
		return SendTaskResult{}, operationError(s.logger, "target agent is not available", err)
	}

	taskID, err := generateTaskID()
	if err != nil {
		return SendTaskResult{}, operationError(s.logger, "failed to generate task id", err)
	}

	sendMessage := cmd.Message
	if cmd.IncludeResponseInstructions && !strings.Contains(cmd.Message, "応答方法：send_response MCPツール") {
		sendMessage = cmd.Message + "\n\n---\n" + buildResponseInstruction(taskID)
	}

	taskLabel := cmd.TaskLabel
	if taskLabel == "" {
		taskLabel = truncate(cmd.Message, 50)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	task := domain.Task{
		ID:             taskID,
		AgentName:      cmd.AgentName,
		AssigneePaneID: agent.PaneID,
		SenderName:     cmd.FromAgent,
		Label:          taskLabel,
		Status:         "pending",
		SentAt:         now,
	}
	if err := s.tasks.CreateTask(ctx, task); err != nil {
		return SendTaskResult{}, operationError(s.logger, "failed to persist task", err)
	}

	if err := s.sender.SendKeys(ctx, agent.PaneID, sendMessage); err != nil {
		failNotes := truncate("delivery failed", 120)
		if failErr := s.tasks.MarkTaskFailed(ctx, taskID, failNotes); failErr != nil {
			logf(s.logger, "mark task %s failed: %v", taskID, failErr)
			return SendTaskResult{}, operationError(s.logger, "message delivery failed; task may remain pending", fmt.Errorf("%w (mark task failed: %v)", err, failErr))
		}
		return SendTaskResult{}, operationError(s.logger, "message delivery failed", err)
	}

	result := SendTaskResult{
		TaskID:    taskID,
		AgentName: cmd.AgentName,
		PaneID:    agent.PaneID,
		SentAt:    now,
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

func generateTaskID() (string, error) {
	b := make([]byte, 6)
	if _, err := randRead(b); err != nil {
		return "", fmt.Errorf("generate task id: %w", err)
	}
	return "t-" + hex.EncodeToString(b), nil
}

// truncate は文字列を切り詰める。改行はスペースに変換する。
func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len([]rune(s)) > maxLen {
		return string([]rune(s)[:maxLen]) + "..."
	}
	return s
}
