package mcptool

import (
	"context"
	"errors"
	"fmt"

	"myT-x/internal/mcp/agent-orchestrator/internal/mcp"
	"myT-x/internal/mcp/agent-orchestrator/usecase"
)

// Handler は MCP ツールハンドラを提供する。
type Handler struct {
	agentSvc      *usecase.AgentService
	dispatchSvc   *usecase.TaskDispatchService
	querySvc      *usecase.TaskQueryService
	taskUpdateSvc *usecase.TaskUpdateService
	responseSvc   *usecase.ResponseService
	statusSvc     *usecase.StatusService
	captureSvc    *usecase.CaptureService
	memberSvc     *usecase.MemberBootstrapService
	instanceID    string
}

// NewHandler は Handler を構築する。
func NewHandler(
	agentSvc *usecase.AgentService,
	dispatchSvc *usecase.TaskDispatchService,
	querySvc *usecase.TaskQueryService,
	taskUpdateSvc *usecase.TaskUpdateService,
	responseSvc *usecase.ResponseService,
	statusSvc *usecase.StatusService,
	captureSvc *usecase.CaptureService,
	memberSvc *usecase.MemberBootstrapService,
	instanceID string,
) *Handler {
	return &Handler{
		agentSvc:      agentSvc,
		dispatchSvc:   dispatchSvc,
		querySvc:      querySvc,
		taskUpdateSvc: taskUpdateSvc,
		responseSvc:   responseSvc,
		statusSvc:     statusSvc,
		captureSvc:    captureSvc,
		memberSvc:     memberSvc,
		instanceID:    instanceID,
	}
}

// BuildRegistry registers the orchestrator MCP tools.
func (h *Handler) BuildRegistry() (*mcp.Registry, error) {
	return mcp.NewRegistry([]mcp.Tool{
		h.registerAgentTool(),
		h.listAgentsTool(),
		h.sendTaskTool(),
		h.sendTasksTool(),
		h.getMyTasksTool(),
		h.getTaskMessageTool(),
		h.getTaskDetailTool(),
		h.acknowledgeTaskTool(),
		h.sendResponseTool(),
		h.updateStatusTool(),
		h.getAgentStatusTool(),
		h.checkTasksTool(),
		h.activateReadyTasksTool(),
		h.cancelTaskTool(),
		h.updateTaskProgressTool(),
		h.capturePaneTool(),
		h.addMemberTool(),
		helpTool(),
	})
}

// register_agent: エージェントのペインIDと名前を紐付け、ロール・得意分野を登録する。
// 同名で再呼び出しすると情報を更新する。登録・変更・更新は caller の pane に関係なく実行できる。
// ※ 他ツール利用の前提条件。最初に必ず実行すること。
//
// パラメータ:
//   - name（必須）: エージェント名（英数字・._-、最大64文字）
//   - pane_id（必須）: tmux ペインID（例: "%1"）
//   - role（任意）: 役割（最大120文字）
//   - skills（任意）: 得意分野（最大20件。文字列配列も後方互換で受付可）
func (h *Handler) registerAgentTool() mcp.Tool {
	return mcp.Tool{
		Name:        "register_agent",
		Description: "Register an agent by binding a pane ID to a name with optional role and skills. Re-calling with the same name updates the entry. Can be called regardless of the caller's pane. Prerequisite for all other tools.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":    map[string]any{"type": "string", "description": "Agent name (alphanumeric, '.', '_', '-'; max 64 chars)"},
				"pane_id": map[string]any{"type": "string", "description": "tmux pane ID (e.g. \"%1\")"},
				"role":    map[string]any{"type": "string", "description": "Agent role (max 120 chars)"},
				"skills": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name":        map[string]any{"type": "string", "description": "Skill name (max 100 chars)"},
							"description": map[string]any{"type": "string", "description": "Skill description (max 400 chars)"},
						},
						"required": []string{"name"},
					},
					"description": "Skill list (max 20 items; string array also accepted for backward compatibility)",
				},
			},
			"required": []string{"name", "pane_id"},
		},
		Handler: h.handleRegisterAgent,
	}
}

func (h *Handler) handleRegisterAgent(ctx context.Context, args map[string]any) (any, error) {
	name, err := requiredAgentName(args, "name")
	if err != nil {
		return nil, err
	}
	paneID, err := requiredPaneID(args, "pane_id")
	if err != nil {
		return nil, err
	}
	role, err := optionalBoundedString(args, "role", maxRoleLen)
	if err != nil {
		return nil, err
	}
	skills, err := optionalSkillList(args, "skills", maxSkills)
	if err != nil {
		return nil, err
	}

	result, err := h.agentSvc.Register(ctx, usecase.RegisterAgentCmd{
		Name:          name,
		PaneID:        paneID,
		Role:          role,
		Skills:        skills,
		MCPInstanceID: h.instanceID,
	})
	if err != nil {
		return nil, err
	}

	m := map[string]any{
		"name":       result.Name,
		"pane_id":    result.PaneID,
		"role":       result.Role,
		"skills":     result.Skills,
		"pane_title": result.PaneTitle,
	}
	if result.TitleWarning != "" {
		m["warning"] = result.TitleWarning
	}
	return m, nil
}

// list_agents: 全エージェント情報を取得し、tmux list-panes と突合して全ペインの状態を返す。
// 登録済みエージェントのみ実行可能。
// 戻り値: registered_agents（名前・役割・スキル・status）と unregistered_panes。
func (h *Handler) listAgentsTool() mcp.Tool {
	return mcp.Tool{
		Name:        "list_agents",
		Description: "List all registered agents with their status and unregistered panes. Requires agent registration. Returns: registered_agents (name, role, skills, status) and unregistered_panes.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: h.handleListAgents,
	}
}

func (h *Handler) handleListAgents(ctx context.Context, _ map[string]any) (any, error) {
	result, err := h.agentSvc.List(ctx)
	if err != nil {
		return nil, err
	}

	registered := make([]map[string]any, 0, len(result.Agents))
	for _, a := range result.Agents {
		registered = append(registered, map[string]any{
			"name":    a.Name,
			"pane_id": a.PaneID,
			"role":    a.Role,
			"skills":  a.Skills,
			"status":  a.Status,
		})
	}
	unregistered := result.Unregistered
	if unregistered == nil {
		unregistered = []string{}
	}

	m := map[string]any{
		"registered_agents":  registered,
		"unregistered_panes": unregistered,
	}
	if result.Orchestrator != nil {
		m["orchestrator"] = map[string]any{"pane_id": result.Orchestrator.PaneID}
	}
	if result.Warning != "" {
		m["warning"] = result.Warning
	}
	return m, nil
}

// send_task: 指定エージェントにタスクを送信する。誰でも送信可能。
// from_agent は自分の登録済みエージェント名を指定（相手が send_response で返信する際の宛先になる）。
// depends_on を指定した場合は blocked で作成され、メッセージ送信は保留される。
//
// パラメータ:
//   - agent_name（必須）: 宛先エージェント名
//   - from_agent（必須）: 自分の登録済みエージェント名（相手が send_response で返信する際の宛先）
//   - message（必須）: 送信メッセージ（最大8000文字）
//   - include_response_instructions（任意）: 応答方法テンプレートを末尾に自動付与（デフォルト: true）
//   - expires_after_minutes（任意）: タスク有効期限（分、1-1440）
//   - depends_on（任意）: 依存タスクID配列（最大20件）。blocked で作成され、依存完了後に activate_ready_tasks で活性化
func (h *Handler) sendTaskTool() mcp.Tool {
	return mcp.Tool{
		Name:        "send_task",
		Description: "Send a task to a target agent. Any registered agent can send. The from_agent value must be your registered agent name so the assignee can reply via send_response. With depends_on, the task is created as blocked and activation is deferred until dependencies complete.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"agent_name":                    map[string]any{"type": "string", "description": "Target agent name"},
				"from_agent":                    map[string]any{"type": "string", "description": "Sender's registered agent name (used as reply-to address for send_response)"},
				"message":                       map[string]any{"type": "string", "description": "Task message (max 8000 chars)"},
				"include_response_instructions": map[string]any{"type": "boolean", "description": "Auto-append response instructions template (default: true)"},
				"expires_after_minutes":         map[string]any{"type": "integer", "description": "Task expiry in minutes (1-1440)"},
				"depends_on": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Dependency task ID array (max 20). Task created as blocked; activate via activate_ready_tasks after deps complete",
				},
			},
			"required": []string{"agent_name", "from_agent", "message"},
		},
		Handler: h.handleSendTask,
	}
}

func (h *Handler) handleSendTask(ctx context.Context, args map[string]any) (any, error) {
	agentName, err := requiredAgentName(args, "agent_name")
	if err != nil {
		return nil, err
	}
	fromAgent, err := requiredAgentName(args, "from_agent")
	if err != nil {
		return nil, err
	}
	message, err := requiredMessage(args, "message")
	if err != nil {
		return nil, err
	}
	includeInstructions, err := optionalBool(args, "include_response_instructions", true)
	if err != nil {
		return nil, err
	}
	expiresAfterMinutes, err := optionalIntBounded(args, "expires_after_minutes", 0, 1, 1440)
	if err != nil {
		return nil, err
	}
	dependsOn, err := optionalTaskIDList(args, "depends_on", maxDependsOnTasks)
	if err != nil {
		return nil, err
	}

	result, err := h.dispatchSvc.Send(ctx, usecase.SendTaskCmd{
		AgentName:                   agentName,
		FromAgent:                   fromAgent,
		Message:                     message,
		IncludeResponseInstructions: includeInstructions,
		ExpiresAfterMinutes:         expiresAfterMinutes,
		DependsOn:                   dependsOn,
		SenderInstanceID:            h.instanceID,
	})
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"task_id":        result.TaskID,
		"agent_name":     result.AgentName,
		"pane_id":        result.PaneID,
		"sender_pane_id": result.SenderPaneID,
		"sent_at":        result.SentAt,
	}, nil
}

// send_tasks: 複数エージェントへ一括でタスクを送信し、group_id でまとめる。
// 登録済みエージェントのみ実行可能。
// 各要素は agent_name / message / include_response_instructions / expires_after_minutes を持つ。
//
// パラメータ:
//   - from_agent（必須）: 自分の登録済みエージェント名（相手が send_response で返信する際の宛先）
//   - group_label（任意）: グループラベル（最大120文字）
//   - tasks（必須）: 送信対象の配列（1-10件）
func (h *Handler) sendTasksTool() mcp.Tool {
	return mcp.Tool{
		Name:        "send_tasks",
		Description: "Batch-send tasks to multiple agents, grouped by group_id. Requires agent registration. Each task item accepts agent_name, message, include_response_instructions, and expires_after_minutes.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"from_agent":  map[string]any{"type": "string", "description": "Sender's registered agent name (used as reply-to address for send_response)"},
				"group_label": map[string]any{"type": "string", "description": "Group label (max 120 chars)"},
				"tasks": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"agent_name":                    map[string]any{"type": "string", "description": "Target agent name"},
							"message":                       map[string]any{"type": "string", "description": "Task message (max 8000 chars)"},
							"include_response_instructions": map[string]any{"type": "boolean", "description": "Auto-append response instructions template (default: true)"},
							"expires_after_minutes":         map[string]any{"type": "integer", "description": "Task expiry in minutes (1-1440)"},
						},
						"required": []string{"agent_name", "message"},
					},
					"description": "Task array (1-10 items)",
				},
			},
			"required": []string{"from_agent", "tasks"},
		},
		Handler: h.handleSendTasks,
	}
}

func (h *Handler) handleSendTasks(ctx context.Context, args map[string]any) (any, error) {
	fromAgent, err := requiredAgentName(args, "from_agent")
	if err != nil {
		return nil, err
	}
	groupLabel, err := optionalBoundedString(args, "group_label", maxGroupLabelLen)
	if err != nil {
		return nil, err
	}
	tasks, err := requiredBatchTasks(args, "tasks")
	if err != nil {
		return nil, err
	}

	result, err := h.dispatchSvc.SendBatch(ctx, usecase.SendTasksCmd{
		FromAgent:        fromAgent,
		Tasks:            tasks,
		GroupLabel:       groupLabel,
		SenderInstanceID: h.instanceID,
	})
	if err != nil {
		return nil, err
	}

	entries := make([]map[string]any, 0, len(result.Results))
	for _, item := range result.Results {
		entry := map[string]any{
			"agent_name": item.AgentName,
		}
		if item.TaskID != "" {
			entry["task_id"] = item.TaskID
		}
		if item.Error != "" {
			entry["error"] = item.Error
		}
		entries = append(entries, entry)
	}

	return map[string]any{
		"group_id": result.GroupID,
		"results":  entries,
		"summary": map[string]any{
			"sent":   result.Summary.Sent,
			"failed": result.Summary.Failed,
		},
	}, nil
}

// get_task_detail: 単一タスクの詳細状態を取得する。
// 送信者・担当者・trusted caller が実行可能。completed タスクは保存済み response を含む。
// メッセージ本文は含まない（本文取得には get_task_message を使う）。
//
// パラメータ:
//   - task_id（必須）: 取得対象の task_id
func (h *Handler) getTaskDetailTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_task_detail",
		Description: "Get detailed task state. Accessible by sender, assignee, or trusted caller. Completed tasks include the stored response. Does not include the message body; use get_task_message for that.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id": map[string]any{"type": "string", "description": "Target task_id"},
			},
			"required": []string{"task_id"},
		},
		Handler: h.handleGetTaskDetail,
	}
}

func (h *Handler) handleGetTaskDetail(ctx context.Context, args map[string]any) (any, error) {
	taskID, err := requiredTaskID(args, "task_id")
	if err != nil {
		return nil, err
	}

	result, err := h.querySvc.GetTaskDetail(ctx, usecase.GetTaskDetailCmd{TaskID: taskID})
	if err != nil {
		return nil, err
	}

	entry := map[string]any{
		"task_id":    result.TaskID,
		"status":     result.Status,
		"agent_name": result.AgentName,
	}
	if result.CompletedAt != "" {
		entry["completed_at"] = result.CompletedAt
	}
	if result.AcknowledgedAt != "" {
		entry["acknowledged_at"] = result.AcknowledgedAt
	}
	if result.CancelledAt != "" {
		entry["cancelled_at"] = result.CancelledAt
	}
	if result.CancelReason != "" {
		entry["cancel_reason"] = result.CancelReason
	}
	if result.ProgressPct != nil {
		entry["progress_pct"] = *result.ProgressPct
	}
	if result.ProgressNote != "" {
		entry["progress_note"] = result.ProgressNote
	}
	if result.ProgressUpdatedAt != "" {
		entry["progress_updated_at"] = result.ProgressUpdatedAt
	}
	if result.ExpiresAt != "" {
		entry["expires_at"] = result.ExpiresAt
	}
	if len(result.DependsOn) > 0 {
		entry["depends_on"] = result.DependsOn
	}
	if result.Response != nil {
		entry["response"] = map[string]any{
			"content":    result.Response.Content,
			"created_at": result.Response.CreatedAt,
		}
	}
	return entry, nil
}

// acknowledge_task: 担当タスクの受領を記録する（任意）。
// 送信者にタスクを認識したことを伝える。task assignee のみ実行可能。
//
// パラメータ:
//   - agent_name（必須）: 自分のエージェント名
//   - task_id（必須）: 受領する task_id
func (h *Handler) acknowledgeTaskTool() mcp.Tool {
	return mcp.Tool{
		Name:        "acknowledge_task",
		Description: "Record task acknowledgment (optional). Signals to the sender that the task is recognized. Assignee only.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"agent_name": map[string]any{"type": "string", "description": "Your agent name"},
				"task_id":    map[string]any{"type": "string", "description": "Task ID to acknowledge"},
			},
			"required": []string{"agent_name", "task_id"},
		},
		Handler: h.handleAcknowledgeTask,
	}
}

func (h *Handler) handleAcknowledgeTask(ctx context.Context, args map[string]any) (any, error) {
	agentName, err := requiredAgentName(args, "agent_name")
	if err != nil {
		return nil, err
	}
	taskID, err := requiredTaskID(args, "task_id")
	if err != nil {
		return nil, err
	}

	result, err := h.taskUpdateSvc.AcknowledgeTask(ctx, usecase.AcknowledgeTaskCmd{
		AgentName: agentName,
		TaskID:    taskID,
	})
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"task_id":         result.TaskID,
		"agent_name":      result.AgentName,
		"acknowledged_at": result.AcknowledgedAt,
	}, nil
}

// get_my_tasks: 自分宛のタスク一覧を取得する（受信箱）。
// 呼び出し元の登録名と agent_name が一致する場合のみ返す。
// status_filter 省略時は pending のみ。blocked / cancelled / expired も指定可能。
// 戻り値に response_instructions（返信手順）を含む。
//
// パラメータ:
//   - agent_name（必須）: 自分のエージェント名
//   - status_filter（任意）: "pending" / "blocked" / "completed" / "all" / "failed" / "abandoned" / "cancelled" / "expired"
func (h *Handler) getMyTasksTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_my_tasks",
		Description: "Get tasks assigned to you (inbox). Only returns tasks where caller matches agent_name. Default filter: pending; blocked, completed, all, failed, abandoned, cancelled, and expired are also supported. Pending unacknowledged tasks are returned with inline message content and auto-acknowledged. Task and inline message entries include from_agent when available. Response includes response_instructions.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"agent_name":    map[string]any{"type": "string", "description": "Your agent name"},
				"status_filter": map[string]any{"type": "string", "description": "\"pending\" / \"blocked\" / \"completed\" / \"all\" / \"failed\" / \"abandoned\" / \"cancelled\" / \"expired\""},
				"max_inline":    map[string]any{"type": "integer", "description": fmt.Sprintf("Max number of inline messages to include (default: %d, range: 1-10)", usecase.DefaultMaxInline)},
			},
			"required": []string{"agent_name"},
		},
		Handler: h.handleGetMyTasks,
	}
}

func (h *Handler) handleGetMyTasks(ctx context.Context, args map[string]any) (any, error) {
	agentName, err := requiredAgentName(args, "agent_name")
	if err != nil {
		return nil, err
	}
	statusFilter, err := optionalStatusFilter(args, "status_filter", "pending")
	if err != nil {
		return nil, err
	}
	maxInline, err := optionalIntBounded(args, "max_inline", usecase.DefaultMaxInline, 1, 10)
	if err != nil {
		return nil, err
	}

	result, err := h.querySvc.GetMyTasks(ctx, usecase.GetMyTasksCmd{
		AgentName:    agentName,
		StatusFilter: statusFilter,
		MaxInline:    maxInline,
	})
	if err != nil {
		return nil, err
	}

	taskList := make([]map[string]any, 0, len(result.Tasks))
	for _, t := range result.Tasks {
		entry := map[string]any{
			"task_id":        t.TaskID,
			"status":         t.Status,
			"sent_at":        t.SentAt,
			"is_now_session": t.IsNowSession,
		}
		if t.FromAgent != "" {
			entry["from_agent"] = t.FromAgent
		}
		if t.SenderPaneID != "" {
			entry["sender_pane_id"] = t.SenderPaneID
		}
		if t.SendMessageID != "" {
			entry["send_message_id"] = t.SendMessageID
		}
		if t.CompletedAt != "" {
			entry["completed_at"] = t.CompletedAt
		}
		taskList = append(taskList, entry)
	}

	resp := map[string]any{
		"agent_name":            result.AgentName,
		"tasks":                 taskList,
		"response_instructions": result.ResponseInstructions,
	}

	if len(result.InlineMessages) > 0 {
		inlineList := make([]map[string]any, 0, len(result.InlineMessages))
		for _, m := range result.InlineMessages {
			inlineList = append(inlineList, map[string]any{
				"task_id":         m.TaskID,
				"send_message_id": m.SendMessageID,
				"content":         m.Content,
				"sent_at":         m.SentAt,
				"from_agent":      m.FromAgent,
			})
		}
		resp["inline_messages"] = inlineList
	}

	return resp, nil
}

// get_task_message: send_message_id からタスクのメッセージ本文とメタデータを取得する。
// 呼び出し元の登録名と agent_name が一致する場合のみ返す。
// get_my_tasks で取得した send_message_id を指定して使う。
// タスクの進捗・依存関係・応答内容の確認には get_task_detail を使う。
//
// パラメータ:
//   - agent_name（必須）: 自分のエージェント名
//   - send_message_id（必須）: 取得対象の send_message_id（m- プレフィックス）
func (h *Handler) getTaskMessageTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_task_message",
		Description: "Get task message body and metadata by send_message_id. Only accessible by the assignee. Use get_task_detail for progress/dependencies/responses.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"agent_name":      map[string]any{"type": "string", "description": "Your agent name"},
				"send_message_id": map[string]any{"type": "string", "description": "Target send_message_id (m- prefix)"},
			},
			"required": []string{"agent_name", "send_message_id"},
		},
		Handler: h.handleGetTaskMessage,
	}
}

func (h *Handler) handleGetTaskMessage(ctx context.Context, args map[string]any) (any, error) {
	agentName, err := requiredAgentName(args, "agent_name")
	if err != nil {
		return nil, err
	}
	sendMessageID, err := requiredSendMessageID(args, "send_message_id")
	if err != nil {
		return nil, err
	}

	result, err := h.querySvc.GetTaskMessage(ctx, usecase.GetTaskMessageCmd{
		AgentName:     agentName,
		SendMessageID: sendMessageID,
	})
	if err != nil {
		return nil, err
	}

	entry := map[string]any{
		"task_id":         result.TaskID,
		"agent_name":      result.AgentName,
		"send_message_id": result.SendMessageID,
		"status":          result.Status,
		"sent_at":         result.SentAt,
		"is_now_session":  result.IsNowSession,
		"message": map[string]any{
			"content":    result.Message.Content,
			"created_at": result.Message.CreatedAt,
		},
	}
	if result.SenderPaneID != "" {
		entry["sender_pane_id"] = result.SenderPaneID
	}
	if result.CompletedAt != "" {
		entry["completed_at"] = result.CompletedAt
	}

	return entry, nil
}

// send_response: タスク送信者にメッセージを返信し、対象タスクを completed に更新する。
// pending 状態の task_id を持つ担当者のみ実行可能。task_id を省略するとエラー。
// 送信者のペインにメッセージが送られる。
//
// パラメータ:
//   - task_id（必須）: 応答対象の task_id
//   - message（必須）: 返信メッセージ（最大8000文字）
func (h *Handler) sendResponseTool() mcp.Tool {
	return mcp.Tool{
		Name:        "send_response",
		Description: "Reply to the task sender and mark the task as completed. Assignee of a pending task only. task_id is required.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string", "description": "Reply message (max 8000 chars)"},
				"task_id": map[string]any{"type": "string", "description": "Task ID to respond to"},
			},
			"required": []string{"message", "task_id"},
		},
		Handler: h.handleSendResponse,
	}
}

// update_status: 自分のエージェント状態を更新する。
// status は idle（待機中・タスク受付可）/ busy（作業中・新規タスク受付不可）/ working（作業中・新規タスク受付可）。
//
// パラメータ:
//   - agent_name（必須）: 自分のエージェント名
//   - status（必須）: "idle" / "busy" / "working"
//   - current_task_id（任意）: 現在作業中の task_id（空文字でクリア）
//   - note（任意）: 補足メモ（最大200文字）
func (h *Handler) updateStatusTool() mcp.Tool {
	return mcp.Tool{
		Name:        "update_status",
		Description: "Update your agent status. Values: idle (accepting tasks), busy (no new tasks), working (active but accepting tasks). Setting status to idle re-delivers unacknowledged pending tasks and auto-acknowledges successful deliveries to avoid duplicates.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"agent_name":      map[string]any{"type": "string", "description": "Your agent name"},
				"status":          map[string]any{"type": "string", "description": "\"idle\" (available) / \"busy\" (no new tasks) / \"working\" (active, accepting tasks)"},
				"current_task_id": map[string]any{"type": "string", "description": "Current task_id being worked on (empty string to clear)"},
				"note":            map[string]any{"type": "string", "description": "Status note (max 200 chars)"},
			},
			"required": []string{"agent_name", "status"},
		},
		Handler: h.handleUpdateStatus,
	}
}

func (h *Handler) handleUpdateStatus(ctx context.Context, args map[string]any) (any, error) {
	agentName, err := requiredAgentName(args, "agent_name")
	if err != nil {
		return nil, err
	}
	statusValue, err := requiredAgentWorkStatus(args, "status")
	if err != nil {
		return nil, err
	}
	currentTaskID, err := optionalTaskID(args, "current_task_id")
	if err != nil {
		return nil, err
	}
	note, err := optionalBoundedString(args, "note", maxStatusNoteLen)
	if err != nil {
		return nil, err
	}

	result, err := h.statusSvc.UpdateStatus(ctx, usecase.UpdateStatusCmd{
		AgentName:     agentName,
		Status:        statusValue,
		CurrentTaskID: currentTaskID,
		Note:          note,
	})
	if err != nil {
		return nil, err
	}

	resp := map[string]any{
		"agent_name":        result.AgentName,
		"status":            result.Status,
		"updated_at":        result.UpdatedAt,
		"redelivered_count": result.RedeliveredCount,
	}
	return resp, nil
}

// get_agent_status: 特定エージェントの最新ステータスを取得する。
// 登録済みエージェントであれば誰でも実行可能。
//
// パラメータ:
//   - agent_name（必須）: 対象エージェント名
func (h *Handler) getAgentStatusTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_agent_status",
		Description: "Get the latest status of a specific agent. Any registered agent can call this.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"agent_name": map[string]any{"type": "string", "description": "Target agent name"},
			},
			"required": []string{"agent_name"},
		},
		Handler: h.handleGetAgentStatus,
	}
}

func (h *Handler) handleGetAgentStatus(ctx context.Context, args map[string]any) (any, error) {
	agentName, err := requiredAgentName(args, "agent_name")
	if err != nil {
		return nil, err
	}

	result, err := h.statusSvc.GetAgentStatus(ctx, usecase.GetAgentStatusCmd{AgentName: agentName})
	if err != nil {
		return nil, err
	}

	entry := map[string]any{
		"agent_name": result.AgentName,
		"status":     result.Status,
	}
	if result.CurrentTaskID != "" {
		entry["current_task_id"] = result.CurrentTaskID
	}
	if result.Note != "" {
		entry["note"] = result.Note
	}
	if result.SecondsSinceUpdate != nil {
		entry["seconds_since_update"] = *result.SecondsSinceUpdate
	}
	return entry, nil
}

func (h *Handler) handleSendResponse(ctx context.Context, args map[string]any) (any, error) {
	message, err := requiredMessage(args, "message")
	if err != nil {
		return nil, err
	}
	taskID, err := requiredTaskID(args, "task_id")
	if err != nil {
		return nil, err
	}

	result, err := h.responseSvc.Send(ctx, usecase.SendResponseCmd{
		Message: message,
		TaskID:  taskID,
	})
	if err != nil {
		return nil, err
	}

	m := map[string]any{
		"sent_to":      result.SentTo,
		"sent_to_name": result.SentToName,
	}
	if result.Warning != "" {
		m["warning"] = result.Warning
	}
	if result.TaskID != "" {
		m["task_id"] = result.TaskID
		m["task_status"] = result.TaskStatus
		m["completed_at"] = result.CompletedAt
	}
	return m, nil
}

// list_all_tasks: 全タスクの状態一覧を取得する（全体監視用）。
// 登録済みエージェントであれば誰でも実行可能。status_filter 省略時は all。
// 戻り値に summary（pending/blocked/completed/failed/abandoned/cancelled/expired 件数）を含む。
// get_my_tasks は自分宛タスクだけを返す受信箱。
//
// パラメータ:
//   - status_filter（任意）: "pending" / "blocked" / "completed" / "all" / "failed" / "abandoned" / "cancelled" / "expired"
//   - assignee_name（任意）: 担当者（assignee）でフィルタ
func (h *Handler) checkTasksTool() mcp.Tool {
	return mcp.Tool{
		Name:        "list_all_tasks",
		Description: "List all tasks for monitoring. Any registered agent can call. Default filter: all. Response includes a summary with counts for pending, blocked, completed, failed, abandoned, cancelled, and expired. Unlike get_my_tasks, this shows all agents' tasks.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"status_filter": map[string]any{"type": "string", "description": "\"pending\" / \"blocked\" / \"completed\" / \"all\" / \"failed\" / \"abandoned\" / \"cancelled\" / \"expired\""},
				"assignee_name": map[string]any{"type": "string", "description": "Filter by assignee agent name"},
			},
		},
		Handler: h.handleListAllTasks,
	}
}

func (h *Handler) handleListAllTasks(ctx context.Context, args map[string]any) (any, error) {
	statusFilter, err := optionalStatusFilter(args, "status_filter", "all")
	if err != nil {
		return nil, err
	}
	agentName, err := optionalAgentName(args, "assignee_name")
	if err != nil {
		return nil, err
	}

	result, err := h.querySvc.ListAllTasks(ctx, usecase.ListAllTasksCmd{
		StatusFilter: statusFilter,
		AgentName:    agentName,
	})
	if err != nil {
		return nil, err
	}

	taskList := make([]map[string]any, 0, len(result.Tasks))
	for _, t := range result.Tasks {
		entry := map[string]any{
			"task_id":        t.TaskID,
			"agent_name":     t.AgentName,
			"status":         t.Status,
			"sent_at":        t.SentAt,
			"is_now_session": t.IsNowSession,
		}
		if t.SenderPaneID != "" {
			entry["sender_pane_id"] = t.SenderPaneID
		}
		if t.SendMessageID != "" {
			entry["send_message_id"] = t.SendMessageID
		}
		if t.CompletedAt != "" {
			entry["completed_at"] = t.CompletedAt
		}
		taskList = append(taskList, entry)
	}

	return map[string]any{
		"tasks": taskList,
		"summary": map[string]any{
			"pending":   result.Pending,
			"blocked":   result.Blocked,
			"completed": result.Completed,
			"failed":    result.Failed,
			"abandoned": result.Abandoned,
			"cancelled": result.Cancelled,
			"expired":   result.Expired,
		},
	}, nil
}

// cancel_task: 送信済みの pending または blocked タスクをキャンセルする。
// 送信者のみ実行可能。
//
// パラメータ:
//   - task_id（必須）: キャンセルする task_id
//   - reason（任意）: キャンセル理由（最大500文字）
func (h *Handler) cancelTaskTool() mcp.Tool {
	return mcp.Tool{
		Name:        "cancel_task",
		Description: "Cancel a pending or blocked task. Sender only.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id": map[string]any{"type": "string", "description": "Task ID to cancel"},
				"reason":  map[string]any{"type": "string", "description": "Cancellation reason (max 500 chars)"},
			},
			"required": []string{"task_id"},
		},
		Handler: h.handleCancelTask,
	}
}

func (h *Handler) handleCancelTask(ctx context.Context, args map[string]any) (any, error) {
	taskID, err := requiredTaskID(args, "task_id")
	if err != nil {
		return nil, err
	}
	reason, err := optionalBoundedString(args, "reason", maxCancelReasonLen)
	if err != nil {
		return nil, err
	}

	result, err := h.dispatchSvc.CancelTask(ctx, usecase.CancelTaskCmd{
		TaskID: taskID,
		Reason: reason,
	})
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"task_id": result.TaskID,
		"status":  result.Status,
	}, nil
}

// activate_ready_tasks: blocked タスクの依存関係を評価し、全依存が完了したタスクを pending に切り替えて配信する。
// タスク完了後や定期的に呼び出す。登録済みエージェント全員が実行可能。
//
// パラメータ:
//   - assignee_name（任意）: 担当者（assignee）でフィルタ。指定エージェントの blocked タスクのみ評価する
func (h *Handler) activateReadyTasksTool() mcp.Tool {
	return mcp.Tool{
		Name:        "activate_ready_tasks",
		Description: "Evaluate blocked tasks and activate those whose dependencies are all completed. Call after completing tasks or periodically. Any registered agent can call.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"assignee_name": map[string]any{"type": "string", "description": "Filter by assignee (evaluate only this agent's blocked tasks)"},
			},
		},
		Handler: h.handleActivateReadyTasks,
	}
}

func (h *Handler) handleActivateReadyTasks(ctx context.Context, args map[string]any) (any, error) {
	agentName, err := optionalAgentName(args, "assignee_name")
	if err != nil {
		return nil, err
	}
	result, err := h.querySvc.ActivateReadyTasks(ctx, usecase.ActivateReadyTasksCmd{AgentName: agentName})
	if err != nil {
		return nil, err
	}
	activated := make([]map[string]any, 0, len(result.Activated))
	for _, task := range result.Activated {
		activated = append(activated, map[string]any{
			"task_id":    task.TaskID,
			"agent_name": task.AgentName,
		})
	}
	return map[string]any{
		"activated":     activated,
		"still_blocked": result.StillBlocked,
	}, nil
}

// update_task_progress: 担当タスクの進捗率または進捗メモを更新する。
// task assignee のみ実行可能。progress_pct または progress_note のいずれかは必須。
//
// パラメータ:
//   - task_id（必須）: 対象の task_id
//   - progress_pct（任意）: 進捗率（0-100）
//   - progress_note（任意）: 進捗メモ（最大500文字）
func (h *Handler) updateTaskProgressTool() mcp.Tool {
	return mcp.Tool{
		Name:        "update_task_progress",
		Description: "Update progress percentage or note on an assigned task. Assignee only. At least one of progress_pct or progress_note required.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id":       map[string]any{"type": "string", "description": "Target task_id"},
				"progress_pct":  map[string]any{"type": "integer", "description": "Progress percentage (0-100)"},
				"progress_note": map[string]any{"type": "string", "description": "Progress note (max 500 chars)"},
			},
			"required": []string{"task_id"},
		},
		Handler: h.handleUpdateTaskProgress,
	}
}

func (h *Handler) handleUpdateTaskProgress(ctx context.Context, args map[string]any) (any, error) {
	taskID, err := requiredTaskID(args, "task_id")
	if err != nil {
		return nil, err
	}
	progressPct, err := optionalProgressPct(args, "progress_pct")
	if err != nil {
		return nil, err
	}
	progressNote, err := optionalBoundedStringPtr(args, "progress_note", maxProgressNoteLen)
	if err != nil {
		return nil, err
	}
	if progressPct == nil && progressNote == nil {
		return nil, errors.New("progress_pct or progress_note is required")
	}

	result, err := h.taskUpdateSvc.UpdateTaskProgress(ctx, usecase.UpdateTaskProgressCmd{
		TaskID:       taskID,
		ProgressPct:  progressPct,
		ProgressNote: progressNote,
	})
	if err != nil {
		return nil, err
	}

	entry := map[string]any{
		"task_id":             result.TaskID,
		"progress_updated_at": result.ProgressUpdatedAt,
	}
	if result.ProgressPct != nil {
		entry["progress_pct"] = *result.ProgressPct
	}
	return entry, nil
}

// capture_pane: 指定エージェントのペイン表示内容を取得する。
// 登録済みエージェントであれば誰でも実行可能。相手の進捗確認・エラー確認に使用。
// デフォルト50行、最大200行。
//
// パラメータ:
//   - agent_name（必須）: 対象エージェント名
//   - lines（任意）: 取得行数（1-200、デフォルト: 50）
func (h *Handler) capturePaneTool() mcp.Tool {
	return mcp.Tool{
		Name:        "capture_pane",
		Description: "Capture the terminal output of a target agent's pane. Any registered agent can call. Useful for checking progress or errors. Default 50 lines, max 200.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"agent_name": map[string]any{"type": "string", "description": "Target agent name"},
				"lines":      map[string]any{"type": "integer", "description": "Number of lines to capture (1-200, default: 50)"},
			},
			"required": []string{"agent_name"},
		},
		Handler: h.handleCapturePane,
	}
}

func (h *Handler) handleCapturePane(ctx context.Context, args map[string]any) (any, error) {
	agentName, err := requiredAgentName(args, "agent_name")
	if err != nil {
		return nil, err
	}
	lines, err := optionalLines(args, "lines", 50, maxCaptureLines)
	if err != nil {
		return nil, err
	}

	result, err := h.captureSvc.Capture(ctx, usecase.CapturePaneCmd{
		AgentName: agentName,
		Lines:     lines,
	})
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"agent_name": result.AgentName,
		"pane_id":    result.PaneID,
		"lines":      result.Lines,
		"content":    result.Content,
		"warning":    result.Warning,
	}, nil
}

// add_member: 新メンバーを動的に追加する。ペイン分割→CLI起動→ブートストラップメッセージ送信を一括実行。
// 登録済みエージェントのみ実行可能。
//
// パラメータ:
//   - pane_title（必須）: メンバー表示名（最大30文字）
//   - role（必須）: 役割（最大120文字）
//   - command（必須）: CLIコマンド（例: "claude"）（最大100文字）
//   - args（任意）: コマンド引数配列
//   - custom_message（任意）: 追加指示メッセージ（最大2000文字）
//   - skills（任意）: 得意分野配列（register_agentと同形式）
//   - team_name（任意）: チーム名（最大64文字、デフォルト: "動的チーム"）
//   - split_from（任意）: 分割元ペインID（デフォルト: 呼び出し元ペイン）
//   - split_direction（任意）: "horizontal" / "vertical"（デフォルト: "horizontal"）
//   - bootstrap_delay_ms（任意）: CLI起動後の待ち時間ms（1000-30000、デフォルト: 3000）
func (h *Handler) addMemberTool() mcp.Tool {
	return mcp.Tool{
		Name:        "add_member",
		Description: "Dynamically add a new team member by splitting a pane, starting the CLI, and sending a bootstrap message. Requires agent registration.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pane_title":         map[string]any{"type": "string", "description": "Member display name (max 30 chars)"},
				"role":               map[string]any{"type": "string", "description": "Role (max 120 chars)"},
				"command":            map[string]any{"type": "string", "description": "CLI command (e.g. \"claude\") (max 100 chars)"},
				"args":               map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Command arguments array"},
				"custom_message":     map[string]any{"type": "string", "description": "Additional instructions message (max 2000 chars)"},
				"skills":             map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}, "description": map[string]any{"type": "string"}}, "required": []string{"name"}}, "description": "Skill list (same format as register_agent)"},
				"team_name":          map[string]any{"type": "string", "description": fmt.Sprintf("Team name (max 64 chars, default: %q)", usecase.DefaultMemberTeamName)},
				"split_from":         map[string]any{"type": "string", "description": "Source pane ID to split (default: caller's pane)"},
				"split_direction":    map[string]any{"type": "string", "enum": []string{"horizontal", "vertical"}, "description": "Split direction (default: \"horizontal\")"},
				"bootstrap_delay_ms": map[string]any{"type": "integer", "description": "Delay after CLI startup in ms (1000-30000, default: 3000)"},
			},
			"required": []string{"pane_title", "role", "command"},
		},
		Handler: h.handleAddMember,
	}
}

func (h *Handler) handleAddMember(ctx context.Context, args map[string]any) (any, error) {
	if h.memberSvc == nil {
		return nil, errors.New("add_member is not available")
	}
	paneTitle, err := requiredString(args, "pane_title", maxPaneTitleLen)
	if err != nil {
		return nil, err
	}
	role, err := requiredString(args, "role", maxRoleLen)
	if err != nil {
		return nil, err
	}
	command, err := requiredString(args, "command", maxCommandLen)
	if err != nil {
		return nil, err
	}
	cmdArgs, err := optionalStringList(args, "args", maxArgs, maxArgLen)
	if err != nil {
		return nil, err
	}
	customMessage, err := optionalBoundedString(args, "custom_message", maxCustomMessageLen)
	if err != nil {
		return nil, err
	}
	skills, err := optionalSkillList(args, "skills", maxSkills)
	if err != nil {
		return nil, err
	}
	teamName, err := optionalBoundedString(args, "team_name", maxTeamNameLen)
	if err != nil {
		return nil, err
	}
	splitFrom, err := optionalPaneID(args, "split_from")
	if err != nil {
		return nil, err
	}
	splitDirection, err := optionalSplitDirection(args, "split_direction", "horizontal")
	if err != nil {
		return nil, err
	}
	bootstrapDelayMs, err := optionalIntBounded(args, "bootstrap_delay_ms",
		usecase.BootstrapDelayDefault, usecase.BootstrapDelayMin, usecase.BootstrapDelayMax)
	if err != nil {
		return nil, err
	}

	result, err := h.memberSvc.AddMember(ctx, usecase.AddMemberCmd{
		PaneTitle:        paneTitle,
		Role:             role,
		Command:          command,
		Args:             cmdArgs,
		CustomMessage:    customMessage,
		Skills:           skills,
		TeamName:         teamName,
		SplitFrom:        splitFrom,
		SplitDirection:   splitDirection,
		BootstrapDelayMs: bootstrapDelayMs,
	})
	if err != nil {
		return nil, err
	}

	m := map[string]any{
		"pane_id":    result.PaneID,
		"pane_title": result.PaneTitle,
		"agent_name": result.AgentName,
	}
	if len(result.Warnings) > 0 {
		m["warnings"] = result.Warnings
	}
	return m, nil
}

// help: オーケストレーターMCPの使い方ヘルプを返す。
// topic を省略すると全体概要とワークフローを返し、ツール名を指定するとそのツールの詳細ヘルプを返す。
// 登録不要で誰でも利用可能。
//
// パラメータ:
//   - topic（任意）: ヘルプトピック（ツール名を指定。省略時は全体概要）
func helpTool() mcp.Tool {
	return mcp.Tool{
		Name:        "help",
		Description: "Get orchestrator MCP usage help. Omit topic for overview and workflow; specify a tool name for detailed help. Detailed help content is returned in English. No registration required.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"topic": map[string]any{"type": "string", "description": "Help topic (tool name). Omit for overview."},
			},
		},
		Handler: handleHelp,
	}
}

func handleHelp(_ context.Context, args map[string]any) (any, error) {
	topic, err := optionalBoundedString(args, "topic", maxAgentNameLen)
	if err != nil {
		return nil, err
	}

	if topic == "" {
		return helpOverview(), nil
	}

	detail, ok := helpForTool(topic)
	if !ok {
		return map[string]any{
			"error":            fmt.Sprintf("unknown topic %q", topic),
			"available_topics": availableTopics(),
		}, nil
	}

	return detail, nil
}
