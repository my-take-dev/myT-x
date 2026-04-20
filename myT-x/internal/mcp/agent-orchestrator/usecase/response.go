package usecase

import (
	"context"
	"crypto/rand"
	"fmt"
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
	TaskStatus  domain.TaskStatus
	CompletedAt string
	Warning     string
}

// ResponseService はタスクへの応答を管理する。
type ResponseService struct {
	agents   domain.AgentRepository
	tasks    domain.TaskRepository
	messages domain.MessageRepository
	sender   domain.PanePasteSender
	resolver domain.SelfPaneResolver
	payloads *payloadWriter
	logger   *log.Logger
	// randRead is the random byte source for ID generation.
	// Defaults to crypto/rand.Read. Tests inject deterministic sources.
	randRead func([]byte) (int, error)
}

// NewResponseService は ResponseService を構築する。
func NewResponseService(
	agents domain.AgentRepository,
	tasks domain.TaskRepository,
	messages domain.MessageRepository,
	sender domain.PanePasteSender,
	resolver domain.SelfPaneResolver,
	logger *log.Logger,
	projectRoots ...string,
) *ResponseService {
	projectRoot := ""
	if len(projectRoots) > 0 {
		projectRoot = projectRoots[0]
	}
	return &ResponseService{
		agents:   agents,
		tasks:    tasks,
		messages: messages,
		sender:   sender,
		resolver: resolver,
		payloads: newPayloadWriter(projectRoot),
		logger:   ensureLogger(logger),
		randRead: rand.Read,
	}
}

// Send はタスク送信者に応答を返す。
func (s *ResponseService) Send(ctx context.Context, cmd SendResponseCmd) (SendResponseResult, error) {
	logf(s.logger, "send_response start task_id=%s message_length=%d", cmd.TaskID, len([]rune(cmd.Message)))
	caller, err := preflightAssigneeTaskCaller(ctx, s.resolver, s.agents, s.tasks, s.logger)
	if err != nil {
		return SendResponseResult{}, err
	}

	task, err := s.tasks.GetTask(ctx, cmd.TaskID)
	if err != nil {
		return SendResponseResult{}, operationError(s.logger, "task is not available", err)
	}
	authMode, allowed := authorizeAssigneeCaller(task, caller)
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
		return SendResponseResult{}, accessDeniedError("caller is not the task assignee")
	}
	if task.Status != domain.TaskStatusPending {
		logf(s.logger, "send_response rejected non-pending task_id=%s status=%s", task.ID, task.Status)
		return SendResponseResult{}, conflictError("task is not pending")
	}

	target, resolvedBy, err := s.resolveResponseTarget(ctx, task)
	if err != nil {
		return SendResponseResult{}, err
	}
	logf(
		s.logger,
		"send_response target resolved task_id=%s target=%s pane_id=%s resolved_by=%s",
		task.ID,
		target.Name,
		target.PaneID,
		resolvedBy,
	)

	result := SendResponseResult{
		SentTo:     target.PaneID,
		SentToName: target.Name,
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// TaskID is always set: the caller needs it regardless of downstream failures.
	result.TaskID = cmd.TaskID

	respID, idWarning := s.generateResponseID(cmd.TaskID, cmd.Message, now)
	if idWarning != "" {
		result.Warning = idWarning
	}
	preparedResponse, err := s.payloads.PrepareResponse(cmd.TaskID, respID, cmd.Message, now)
	if err != nil {
		return SendResponseResult{}, operationError(s.logger, "failed to prepare response payload", err)
	}

	if domain.IsVirtualPaneID(target.PaneID) {
		logf(s.logger, "send_response skip SendKeys for virtual pane task_id=%s target=%s pane_id=%s", task.ID, target.Name, target.PaneID)
	} else {
		if err := s.sender.SendKeysPaste(ctx, target.PaneID, preparedResponse.deliveryText); err != nil {
			if cleanupErr := preparedResponse.Cleanup(); cleanupErr != nil {
				return SendResponseResult{}, operationError(s.logger, "response delivery failed", fmt.Errorf("%w (cleanup payload artifacts: %v)", err, cleanupErr))
			}
			return SendResponseResult{}, operationError(s.logger, "response delivery failed", err)
		}
		logf(s.logger, "send_response delivered task_id=%s target=%s pane_id=%s", task.ID, target.Name, target.PaneID)
	}

	if err := s.messages.SaveResponse(ctx, preparedResponse.message); err != nil {
		if cleanupErr := preparedResponse.Cleanup(); cleanupErr != nil {
			return SendResponseResult{}, operationError(
				s.logger,
				"response was delivered but could not be persisted; task remains pending",
				fmt.Errorf("%w (cleanup payload artifacts: %v)", err, cleanupErr),
			)
		}
		return SendResponseResult{}, operationError(
			s.logger,
			"response was delivered but could not be persisted; task remains pending",
			err,
		)
	}

	if err := s.tasks.CompleteTask(ctx, cmd.TaskID, respID, now); err != nil {
		rollbackErr := s.messages.DeleteResponse(ctx, respID)
		cleanupErr := preparedResponse.Cleanup()
		if rollbackErr != nil || cleanupErr != nil {
			return SendResponseResult{}, operationError(
				s.logger,
				"response was delivered but task completion update failed",
				fmt.Errorf("%w (rollback response %s: %v, cleanup payload artifacts: %v)", err, respID, rollbackErr, cleanupErr),
			)
		}
		result.Warning = appendWarning(result.Warning, "message delivered but task completion update failed; response persistence was rolled back")
		result.TaskStatus = task.Status
		logf(s.logger, "complete task %s after response: %v", cmd.TaskID, err)
	} else {
		logf(s.logger, "send_response completed task_id=%s completed_at=%s", cmd.TaskID, now)
		result.TaskStatus = domain.TaskStatusCompleted
		result.CompletedAt = now
	}

	return result, nil
}

func (s *ResponseService) resolveResponseTarget(ctx context.Context, task domain.Task) (domain.Agent, string, error) {
	if task.SenderInstanceID != "" {
		target, err := s.agents.GetAgentByMCPInstanceID(ctx, task.SenderInstanceID)
		if err == nil {
			return target, "instance", nil
		}
		logf(
			s.logger,
			"send_response instance target lookup failed task_id=%s sender_instance_id=%s: %v",
			task.ID,
			task.SenderInstanceID,
			err,
		)
	}

	if task.SenderName == "" {
		logf(s.logger, "send_response missing sender task_id=%s", task.ID)
		return domain.Agent{}, "", conflictError("task sender is unknown; cannot deliver response")
	}

	target, err := s.agents.GetAgent(ctx, task.SenderName)
	if err != nil {
		return domain.Agent{}, "", operationError(s.logger, "response target is not available", err)
	}
	return target, "name", nil
}

func (s *ResponseService) generateResponseID(taskID string, message string, now string) (string, string) {
	respID, err := generateIDWith(s.randRead, "r-", "generate response id")
	if err == nil {
		return respID, ""
	}
	logf(s.logger, "generate response id for task %s: %v", taskID, err)
	fallbackSeed := fmt.Sprintf("%s\n%s\n%d\n%s", taskID, now, time.Now().UTC().UnixNano(), message)
	fallback := sha256Hex(fallbackSeed)
	return "r-" + fallback[:12], "response id generation failed; using best-effort fallback"
}

func authorizeResponseCaller(task domain.Task, caller domain.Agent) (string, bool) {
	return authorizeAssigneeCaller(task, caller)
}

func appendWarning(base string, extra string) string {
	if base == "" {
		return extra
	}
	if extra == "" {
		return base
	}
	return base + "; " + extra
}

// authorizeAssigneeCaller authorizes a caller against a task's assignee.
//
// Design intent: trusted callers (pipe bridge connections where TMUX_PANE is
// unresolvable) may act as ANY assignee. This is required to make pipe bridge
// and "direct inter-agent communication" (agent-orchestrator/CLAUDE.md L10)
// work. DO NOT tighten this branch — it will break pipe bridge flows.
// See /ACCEPTED_DESIGN_DECISIONS.md AD-001.
func authorizeAssigneeCaller(task domain.Task, caller domain.Agent) (string, bool) {
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
