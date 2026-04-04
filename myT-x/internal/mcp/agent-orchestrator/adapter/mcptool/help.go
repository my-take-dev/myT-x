package mcptool

import (
	"fmt"
	"sort"

	"myT-x/internal/mcp/agent-orchestrator/usecase"
)

// helpOverview は全体概要のヘルプ情報を返す。
func helpOverview() map[string]any {
	return map[string]any{
		"title":       "Agent Orchestrator MCP Help",
		"description": "This orchestrator MCP server manages task coordination and communication between agents. It provides the foundation for multiple AI agents to work together.",
		"available_tools": []string{
			"register_agent", "list_agents", "send_task", "send_tasks",
			"get_my_tasks", "get_task_message", "get_task_detail", "acknowledge_task", "send_response",
			"update_status", "get_agent_status", "list_all_tasks", "activate_ready_tasks", "cancel_task",
			"update_task_progress", "capture_pane", "add_member", "help",
		},
		"workflow": map[string]any{
			"title": "Typical Workflow",
			"steps": []string{
				"1. register_agent: Register yourself as an agent. This is required before using the other tools.",
				"2. get_my_tasks: Check your pending tasks.",
				"3. get_task_message: Fetch the task message body by send_message_id.",
				"4. acknowledge_task: Record task acknowledgment with task_id when confirmation is needed.",
				"5. Execute the task.",
				"6. send_response: Reply with task_id to complete the task and send the response.",
			},
			"additional": []string{
				"send_task: Send a task to another agent. No orchestrator relay is required.",
				"send_tasks: Send tasks to multiple agents in one call and group them with group_id.",
				"get_task_detail: Inspect detailed task state, progress, and responses.",
				"update_status / get_agent_status: Update and inspect agent availability.",
				"list_all_tasks: Monitor all tasks with optional filters.",
				"activate_ready_tasks: Activate blocked tasks whose dependencies are already resolved.",
				"cancel_task / update_task_progress: Cancel sent tasks or report progress on assigned tasks.",
				"list_agents: Inspect registered agents and unregistered panes.",
				"capture_pane: Capture another agent's pane output for progress or error checks.",
				"add_member: Add a new member dynamically by splitting a pane, launching the CLI, and sending bootstrap instructions.",
			},
		},
		"best_practices": []string{
			"Run register_agent first. The other tools are unavailable until registration is complete.",
			"send_response requires task_id. Omitting it prevents the task from being completed.",
			"Sending with include_response_instructions=true, the default, automatically appends reply instructions for the assignee.",
			"Use get_task_detail when you need fields that are not shown in task list views.",
			"Use send_task for direct coordination with other agents; routing through the orchestrator is unnecessary.",
			"Use capture_pane to inspect another agent's screen when you need progress or error context.",
			"Use add_member to grow the team dynamically; a bootstrap message is sent automatically after creation.",
			"When a task is sent with depends_on, call activate_ready_tasks after the dependency tasks finish.",
		},
		"tip": "Use the topic parameter to retrieve detailed help for a specific tool, for example topic=\"send_task\".",
	}
}

// helpForTool は指定ツールの詳細ヘルプ情報を返す。
// 存在しないツール名の場合は nil, false を返す。
func helpForTool(topic string) (map[string]any, bool) {
	h, ok := toolHelps[topic]
	if !ok {
		return nil, false
	}
	return h, true
}

// availableTopics は help で指定可能なトピック一覧をソート済みで返す。
func availableTopics() []string {
	topics := make([]string, 0, len(toolHelps))
	for k := range toolHelps {
		topics = append(topics, k)
	}
	sort.Strings(topics)
	return topics
}

var toolHelps = map[string]map[string]any{
	"register_agent": {
		"tool":        "register_agent",
		"description": "Bind a pane ID to an agent name and register its role and skills. Re-calling with the same name updates the existing entry.",
		"parameters": map[string]any{
			"name (required)":    "Agent name. Allowed characters: alphanumeric, '.', '_', '-'. Maximum 64 characters.",
			"pane_id (required)": "tmux pane ID, for example \"%1\".",
			"role (optional)":    "Agent role. Maximum 120 characters.",
			"skills (optional)":  "Skill list with up to 20 items. The recommended format is an object array such as [{\"name\":\"Go\",\"description\":\"...\"}].",
		},
		"notes": []string{
			"This is the prerequisite for the rest of the toolset. Run it first.",
			"If another agent is already registered to the same pane_id, that entry is replaced.",
			"Registration and updates are allowed regardless of the caller's pane.",
		},
	},
	"list_agents": {
		"tool":        "list_agents",
		"description": "Return all registered agents and reconcile them with tmux list-panes to show pane coverage.",
		"parameters":  map[string]any{},
		"notes": []string{
			"The response contains registered_agents and unregistered_panes.",
			"Each registered_agents entry includes status values such as idle, busy, working, or unknown.",
			"Only registered agents can call this tool.",
		},
	},
	"send_task": {
		"tool":        "send_task",
		"description": "Send a task to a target agent. A successful call returns task_id, and the assignee uses that task_id when replying.",
		"parameters": map[string]any{
			"agent_name (required)":                    "Target agent name.",
			"from_agent (required)":                    "Your registered agent name. This becomes the reply target when the assignee uses send_response.",
			"message (required)":                       "Task message. Maximum 8000 characters.",
			"include_response_instructions (optional)": "Automatically append the response template. Default: true.",
			"expires_after_minutes (optional)":         "Task expiry in minutes. Range: 1-1440.",
			"depends_on (optional)":                    "Dependency task ID array with up to 20 items. When provided, the task is created as blocked.",
		},
		"notes": []string{
			"Any registered agent can send tasks directly; no orchestrator relay is required.",
			"A successful send returns task_id, and the assignee uses that task_id with send_response.",
			"By default, response instructions are appended to the end of the outgoing message.",
			"When depends_on is used, pass existing task_id values. Unknown dependencies are rejected. After the dependency tasks complete, call activate_ready_tasks.",
			"If message delivery fails, the task is recorded as failed.",
		},
	},
	"send_tasks": {
		"tool":        "send_tasks",
		"description": "Send tasks to multiple agents in one batch and group the results under group_id.",
		"parameters": map[string]any{
			"from_agent (required)":  "Your registered agent name. This becomes the reply target when assignees use send_response.",
			"group_label (optional)": "Group label. Maximum 120 characters.",
			"tasks (required)":       "Task array with 1-10 items. Each item supports agent_name, message, include_response_instructions, and expires_after_minutes.",
		},
		"notes": []string{
			"Only registered agents can call this tool.",
			"Successful items return task_id and agent_name. Failed items return agent_name and error only.",
			"group_id is useful for correlating tasks across the batch. It is omitted when every item fails.",
			"depends_on between items in the same batch is not supported. Use individual send_task calls when dependencies are required.",
		},
	},
	"get_my_tasks": {
		"tool":        "get_my_tasks",
		"description": "Return your assigned tasks together with response instructions. Use it as your inbox view. Pending unacknowledged tasks are returned with inline message content and auto-acknowledged.",
		"parameters": map[string]any{
			"agent_name (required)":    "Your agent name.",
			"status_filter (optional)": "\"pending\" / \"blocked\" / \"completed\" / \"all\" / \"failed\" / \"abandoned\" / \"cancelled\" / \"expired\". Default: pending.",
			"max_inline (optional)":    fmt.Sprintf("Max number of inline messages to include. Range: 1-10. Default: %d.", usecase.DefaultMaxInline),
		},
		"notes": []string{
			"Results are returned only when the caller's registered name matches agent_name.",
			"Pending unacknowledged tasks include inline_messages with full message content. These tasks are auto-acknowledged.",
			"Task entries and inline_messages include from_agent when the sender name is available.",
			"For already-acknowledged tasks, use send_message_id with get_task_message to fetch the message body.",
			"The response also includes response_instructions.",
			"Call this periodically (every 30-60 seconds) when idle to ensure you receive all tasks.",
		},
	},
	"get_task_message": {
		"tool":        "get_task_message",
		"description": "Return the task message body and metadata for a given send_message_id.",
		"parameters": map[string]any{
			"agent_name (required)":      "Your agent name.",
			"send_message_id (required)": "Target send_message_id with the m- prefix.",
		},
		"notes": []string{
			"Use the send_message_id returned by get_my_tasks.",
			"Results are returned only when the caller's registered name matches agent_name.",
			"The response includes message.content and message.created_at.",
			"Use this tool for the message body. Use get_task_detail for progress, dependencies, or response inspection.",
		},
	},
	"get_task_detail": {
		"tool":        "get_task_detail",
		"description": "Return detailed state for a single task.",
		"parameters": map[string]any{
			"task_id (required)": "Target task_id.",
		},
		"notes": []string{
			"The sender, the assignee, and trusted callers can use this tool.",
			"Completed tasks include response.content and response.created_at.",
			"Use it for fields such as acknowledged_at, progress, cancel_reason, expires_at, and depends_on.",
			"The message body is not included. The assignee uses get_task_message to read it.",
		},
	},
	"acknowledge_task": {
		"tool":        "acknowledge_task",
		"description": "Record acknowledgment for an assigned task. This is optional and signals to the sender that the task is recognized.",
		"parameters": map[string]any{
			"agent_name (required)": "Your agent name.",
			"task_id (required)":    "Task ID to acknowledge.",
		},
		"notes": []string{
			"Only the task assignee can call this tool.",
			"The response only includes task_id, agent_name, and acknowledged_at.",
			"Skipping this does not block task execution. The sender can later inspect acknowledged_at through get_task_detail.",
		},
	},
	"send_response": {
		"tool":        "send_response",
		"description": "Reply to the task sender and mark the target task as completed.",
		"parameters": map[string]any{
			"task_id (required)": "Task ID to respond to.",
			"message (required)": "Reply message. Maximum 8000 characters.",
		},
		"notes": []string{
			"Only the assignee of a pending task can call this tool.",
			"Omitting task_id is an error and prevents the task from being completed.",
			"The reply is delivered to the sender's pane and the task becomes completed.",
		},
	},
	"update_status": {
		"tool":        "update_status",
		"description": "Update your current agent status. Setting status to idle triggers automatic re-delivery of unacknowledged pending tasks.",
		"parameters": map[string]any{
			"agent_name (required)":      "Your agent name.",
			"status (required)":          "\"idle\" (available), \"busy\" (working and not accepting new tasks), or \"working\" (working and still accepting new tasks).",
			"current_task_id (optional)": "Current task_id being worked on. Pass an empty string to clear it.",
			"note (optional)":            "Status note. Maximum 200 characters.",
		},
		"notes": []string{
			"You can update only your own status.",
			"When status is set to idle, any unacknowledged pending tasks are automatically re-delivered to your pane. Successful deliveries are auto-acknowledged to avoid duplicate re-delivery loops. The redelivered_count is always included in the response.",
			"Other agents can inspect this through get_agent_status when choosing where to send work.",
			"Call update_status(status=\"idle\") when you finish a task and are waiting for new work.",
		},
	},
	"get_agent_status": {
		"tool":        "get_agent_status",
		"description": "Return the latest status for a specific agent.",
		"parameters": map[string]any{
			"agent_name (required)": "Target agent name.",
		},
		"notes": []string{
			"Any registered agent can call this tool.",
			"note and current_task_id are returned only when values are present.",
		},
	},
	"list_all_tasks": {
		"tool":        "list_all_tasks",
		"description": "Return all task states. This is the monitoring view for team-wide task progress.",
		"parameters": map[string]any{
			"status_filter (optional)": "\"pending\" / \"blocked\" / \"completed\" / \"all\" / \"failed\" / \"abandoned\" / \"cancelled\" / \"expired\". Default: all.",
			"assignee_name (optional)": "Filter by assignee. Only tasks assigned to the specified agent are returned.",
		},
		"notes": []string{
			"Any registered agent can call this tool.",
			"The summary object contains counts for pending, blocked, completed, failed, abandoned, cancelled, and expired.",
			"get_my_tasks is a personal inbox. list_all_tasks is the global monitoring view across all agents.",
		},
	},
	"activate_ready_tasks": {
		"tool":        "activate_ready_tasks",
		"description": "Evaluate blocked tasks and deliver the ones whose dependencies are fully completed by changing them back to pending.",
		"parameters": map[string]any{
			"assignee_name (optional)": "Filter by assignee. Only blocked tasks for the specified agent are evaluated.",
		},
		"notes": []string{
			"Any registered agent can call this tool.",
			"The activated array contains only the tasks that became pending during this call.",
			"still_blocked reports only the number of tasks whose dependencies are still unresolved.",
			"Blocked tasks with cancelled, failed, abandoned, expired, or inconsistent dependencies are automatically normalized to cancelled.",
			"Call this after send_response completes a task to activate any blocked tasks that depended on it.",
		},
	},
	"cancel_task": {
		"tool":        "cancel_task",
		"description": "Cancel a sent task that is still pending or blocked.",
		"parameters": map[string]any{
			"task_id (required)": "Task ID to cancel.",
			"reason (optional)":  "Cancellation reason. Maximum 500 characters.",
		},
		"notes": []string{
			"Only the sender can call this tool.",
			"The response includes task_id and status only.",
		},
	},
	"update_task_progress": {
		"tool":        "update_task_progress",
		"description": "Update progress percentage or a progress note for an assigned task.",
		"parameters": map[string]any{
			"task_id (required)":       "Target task_id.",
			"progress_pct (optional)":  "Progress percentage. Range: 0-100.",
			"progress_note (optional)": "Progress note. Maximum 500 characters.",
		},
		"notes": []string{
			"Only the task assignee can call this tool.",
			"At least one of progress_pct or progress_note is required.",
		},
	},
	"capture_pane": {
		"tool":        "capture_pane",
		"description": "Capture the visible pane output for a target agent.",
		"parameters": map[string]any{
			"agent_name (required)": "Target agent name.",
			"lines (optional)":      "Number of lines to capture. Range: 1-200. Default: 50.",
		},
		"notes": []string{
			"Any registered agent can call this tool.",
			"Use it when you need another agent's progress output or error screen.",
		},
	},
	"add_member": {
		"tool":        "add_member",
		"description": "Add a new member dynamically by splitting a pane, launching the CLI, and sending bootstrap instructions.",
		"parameters": map[string]any{
			"pane_title (required)":         "Member display name. Maximum 30 characters.",
			"role (required)":               "Role. Maximum 120 characters.",
			"command (required)":            "CLI command, for example \"claude\". Maximum 100 characters.",
			"args (optional)":               "Command argument array with up to 20 items.",
			"custom_message (optional)":     "Additional instruction message. Maximum 2000 characters.",
			"skills (optional)":             "Skill list in the same format as register_agent, with up to 20 items.",
			"team_name (optional)":          fmt.Sprintf("Team name. Maximum 64 characters. Default: %q.", usecase.DefaultMemberTeamName),
			"split_from (optional)":         "Source pane ID to split. Default: caller's pane.",
			"split_direction (optional)":    "\"horizontal\" or \"vertical\". Default: \"horizontal\".",
			"bootstrap_delay_ms (optional)": "Delay after CLI startup in milliseconds. Range: 1000-30000. Default: 3000.",
		},
		"notes": []string{
			"Only registered agents can call this tool.",
			"If split_from is omitted, the caller's pane is split.",
			"Claude CLI commands such as claude, claude.exe, and claude-code* are sent in bracketed paste mode.",
			"Pane title failures or bootstrap send failures are returned as warnings and do not stop the overall flow.",
			"The new member automatically receives bootstrap instructions, including the register_agent step.",
		},
	},
	"help": {
		"tool":        "help",
		"description": "Return usage help for the orchestrator MCP server.",
		"parameters": map[string]any{
			"topic (optional)": "Help topic. Pass a tool name for detailed help. Omit it for the overview.",
		},
		"notes": []string{
			"No registration is required.",
			"Omitting topic returns the overview and workflow guidance.",
			"Specifying a tool name returns the detailed help for that tool.",
		},
	},
}
