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
	agentSvc    *usecase.AgentService
	dispatchSvc *usecase.TaskDispatchService
	querySvc    *usecase.TaskQueryService
	responseSvc *usecase.ResponseService
	captureSvc  *usecase.CaptureService
	memberSvc   *usecase.MemberBootstrapService
	instanceID  string
}

// NewHandler は Handler を構築する。
func NewHandler(
	agentSvc *usecase.AgentService,
	dispatchSvc *usecase.TaskDispatchService,
	querySvc *usecase.TaskQueryService,
	responseSvc *usecase.ResponseService,
	captureSvc *usecase.CaptureService,
	memberSvc *usecase.MemberBootstrapService,
	instanceID string,
) *Handler {
	return &Handler{
		agentSvc:    agentSvc,
		dispatchSvc: dispatchSvc,
		querySvc:    querySvc,
		responseSvc: responseSvc,
		captureSvc:  captureSvc,
		memberSvc:   memberSvc,
		instanceID:  instanceID,
	}
}

// BuildRegistry は全10ツールを定義し、Registry を返す。
func (h *Handler) BuildRegistry() (*mcp.Registry, error) {
	return mcp.NewRegistry([]mcp.Tool{
		h.registerAgentTool(),
		h.listAgentsTool(),
		h.sendTaskTool(),
		h.getMyTasksTool(),
		h.getMyTaskTool(),
		h.sendResponseTool(),
		h.checkTasksTool(),
		h.capturePaneTool(),
		h.addMemberTool(),
		helpTool(),
	})
}

func (h *Handler) registerAgentTool() mcp.Tool {
	return mcp.Tool{
		Name:        "register_agent",
		Description: "エージェントのペインIDと名前を紐付け、ロール・得意分野をSQLiteに記録する。同名で再呼び出しすると情報を更新する。登録・変更・更新は caller の pane に関係なく実行できる。※ 他ツール利用の前提条件。最初に必ず実行すること。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":    map[string]any{"type": "string", "description": "エージェント名（英数字・._-、最大64文字）"},
				"pane_id": map[string]any{"type": "string", "description": "tmux ペインID（例: \"%1\"）"},
				"role":    map[string]any{"type": "string", "description": "役割（最大120文字）"},
				"skills": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name":        map[string]any{"type": "string", "description": "スキル名（最大100文字）"},
							"description": map[string]any{"type": "string", "description": "スキル説明（最大400文字）"},
						},
						"required": []string{"name"},
					},
					"description": "得意分野（最大20件。文字列配列も後方互換で受付可）",
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

func (h *Handler) listAgentsTool() mcp.Tool {
	return mcp.Tool{
		Name:        "list_agents",
		Description: "全エージェント情報を取得し、tmux list-panes と突合して全ペインの状態を返す。登録済みエージェントのみ実行可能。戻り値: registered_agents（名前・役割・スキル）と unregistered_panes。",
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

func (h *Handler) sendTaskTool() mcp.Tool {
	return mcp.Tool{
		Name:        "send_task",
		Description: "エージェント間でメッセージを送信し、SQLiteにタスクを記録する。誰でも送信可能。from_agent は自分の名前を指定（返信先として使う登録済みエージェント名）。デフォルトでメッセージ末尾に応答テンプレートを自動付与する。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"agent_name":                    map[string]any{"type": "string", "description": "宛先エージェント名"},
				"from_agent":                    map[string]any{"type": "string", "description": "送信元エージェント名。返信先として使う登録済みエージェント名"},
				"message":                       map[string]any{"type": "string", "description": "送信メッセージ（最大8000文字）"},
				"include_response_instructions": map[string]any{"type": "boolean", "description": "応答方法テンプレートを末尾に自動付与する（デフォルト: true）"},
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

	result, err := h.dispatchSvc.Send(ctx, usecase.SendTaskCmd{
		AgentName:                   agentName,
		FromAgent:                   fromAgent,
		Message:                     message,
		IncludeResponseInstructions: includeInstructions,
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

func (h *Handler) getMyTasksTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_my_tasks",
		Description: "自分宛のタスク情報と応答方法をSQLiteから取得する。呼び出し元の登録名と agent_name が一致する場合のみ返す。status_filter 省略時は pending のみ。戻り値に response_instructions（返信手順）を含む。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"agent_name":    map[string]any{"type": "string", "description": "自分のエージェント名"},
				"status_filter": map[string]any{"type": "string", "description": "\"pending\" / \"completed\" / \"all\" / \"failed\" / \"abandoned\""},
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

	result, err := h.querySvc.GetMyTasks(ctx, usecase.GetMyTasksCmd{
		AgentName:    agentName,
		StatusFilter: statusFilter,
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
		"agent_name":            result.AgentName,
		"tasks":                 taskList,
		"response_instructions": result.ResponseInstructions,
	}, nil
}

func (h *Handler) getMyTaskTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_my_task",
		Description: "send_message_id から自分宛タスクのメッセージ本文とメタデータを取得する。呼び出し元の登録名と agent_name が一致する場合のみ返す。get_my_tasks で取得した send_message_id を指定して使う。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"agent_name":      map[string]any{"type": "string", "description": "自分のエージェント名"},
				"send_message_id": map[string]any{"type": "string", "description": "取得対象の send_message_id（m- プレフィックス）"},
			},
			"required": []string{"agent_name", "send_message_id"},
		},
		Handler: h.handleGetMyTask,
	}
}

func (h *Handler) handleGetMyTask(ctx context.Context, args map[string]any) (any, error) {
	agentName, err := requiredAgentName(args, "agent_name")
	if err != nil {
		return nil, err
	}
	sendMessageID, err := requiredSendMessageID(args, "send_message_id")
	if err != nil {
		return nil, err
	}

	result, err := h.querySvc.GetMyTask(ctx, usecase.GetMyTaskCmd{
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

func (h *Handler) sendResponseTool() mcp.Tool {
	return mcp.Tool{
		Name:        "send_response",
		Description: "タスク送信者にメッセージを返信し、対象タスクを completed に更新する。pending 状態の task_id を持つ担当者のみ実行可能。task_id を省略するとエラー。送信者のペインにメッセージが送られる。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string", "description": "返信メッセージ（最大8000文字）"},
				"task_id": map[string]any{"type": "string", "description": "対応する task_id。必須"},
			},
			"required": []string{"message", "task_id"},
		},
		Handler: h.handleSendResponse,
	}
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

func (h *Handler) checkTasksTool() mcp.Tool {
	return mcp.Tool{
		Name:        "check_tasks",
		Description: "全タスクの状態をSQLiteから取得する。登録済みエージェントであれば誰でも実行可能。status_filter 省略時は all。戻り値に summary（pending/completed/failed/abandoned 件数）を含む。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"status_filter": map[string]any{"type": "string", "description": "\"pending\" / \"completed\" / \"all\" / \"failed\" / \"abandoned\""},
				"agent_name":    map[string]any{"type": "string", "description": "特定エージェントのタスクのみ取得"},
			},
		},
		Handler: h.handleCheckTasks,
	}
}

func (h *Handler) handleCheckTasks(ctx context.Context, args map[string]any) (any, error) {
	statusFilter, err := optionalStatusFilter(args, "status_filter", "all")
	if err != nil {
		return nil, err
	}
	agentName, err := optionalAgentName(args, "agent_name")
	if err != nil {
		return nil, err
	}

	result, err := h.querySvc.CheckTasks(ctx, usecase.CheckTasksCmd{
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
			"completed": result.Completed,
			"failed":    result.Failed,
			"abandoned": result.Abandoned,
		},
	}, nil
}

func (h *Handler) capturePaneTool() mcp.Tool {
	return mcp.Tool{
		Name:        "capture_pane",
		Description: "指定エージェントのペイン表示内容を取得する。登録済みエージェントであれば誰でも実行可能。相手の進捗確認・エラー確認に使用。デフォルト50行、最大200行。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"agent_name": map[string]any{"type": "string", "description": "対象エージェント名"},
				"lines":      map[string]any{"type": "integer", "description": "取得行数（1-200、デフォルト: 50）"},
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

func (h *Handler) addMemberTool() mcp.Tool {
	return mcp.Tool{
		Name:        "add_member",
		Description: "新メンバーを動的に追加する。ペイン分割→CLI起動→ブートストラップメッセージ送信を一括実行。登録済みエージェントのみ実行可能。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pane_title":         map[string]any{"type": "string", "description": "メンバー表示名（最大30文字）"},
				"role":               map[string]any{"type": "string", "description": "役割（最大120文字）"},
				"command":            map[string]any{"type": "string", "description": "CLIコマンド（例: \"claude\"）（最大100文字）"},
				"args":               map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "コマンド引数配列"},
				"custom_message":     map[string]any{"type": "string", "description": "追加指示メッセージ（最大2000文字）"},
				"skills":             map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}, "description": map[string]any{"type": "string"}}, "required": []string{"name"}}, "description": "得意分野配列（register_agentと同形式）"},
				"team_name":          map[string]any{"type": "string", "description": "チーム名（最大64文字、デフォルト: \"動的チーム\"）"},
				"split_from":         map[string]any{"type": "string", "description": "分割元ペインID（デフォルト: 呼び出し元ペイン）"},
				"split_direction":    map[string]any{"type": "string", "enum": []string{"horizontal", "vertical"}, "description": "分割方向（デフォルト: \"horizontal\"）"},
				"bootstrap_delay_ms": map[string]any{"type": "integer", "description": "CLI起動後の待ち時間ms（1000-30000、デフォルト: 3000）"},
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

func helpTool() mcp.Tool {
	return mcp.Tool{
		Name:        "help",
		Description: "オーケストレーターMCPの使い方ヘルプを返す。topic を省略すると全体概要とワークフローを返し、ツール名を指定するとそのツールの詳細ヘルプを返す。登録不要で誰でも利用可能。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"topic": map[string]any{"type": "string", "description": "ヘルプトピック（ツール名を指定。省略時は全体概要）"},
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
