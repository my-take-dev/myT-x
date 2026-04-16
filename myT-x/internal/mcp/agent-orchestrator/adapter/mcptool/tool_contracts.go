package mcptool

type helpToolContract struct {
	registryDescription string
	helpDescription     string
	notes               []string
}

var (
	sendTaskHelpContract = helpToolContract{
		registryDescription: "Send a task to a target agent. Caller-pane registration is optional, but from_agent must resolve to a registered agent so the assignee can reply via send_response. With depends_on, the task is created as blocked and activation is deferred until dependencies complete.",
		helpDescription:     "Send a task to a target agent. A successful call returns task_id, and the assignee uses that task_id when replying.",
		notes: []string{
			"The caller pane does not need to be registered. The from_agent value must resolve to a registered sender.",
			"A successful send returns task_id, and the assignee uses that task_id with send_response.",
			"By default, response instructions are appended to the end of the outgoing message.",
			"When depends_on is used, pass existing task_id values. Unknown dependencies are rejected. After the dependency tasks complete, call activate_ready_tasks.",
			"If message delivery fails, the task is recorded as failed.",
		},
	}
	sendTasksHelpContract = helpToolContract{
		registryDescription: "Batch-send tasks to multiple agents, grouped by group_id. Requires agent registration or trusted access. Each task item accepts agent_name, message, include_response_instructions, and expires_after_minutes.",
		helpDescription:     "Send tasks to multiple agents in one batch and group the results under group_id.",
		notes: []string{
			"Registered agents and trusted callers can use this tool.",
			"Successful items return task_id and agent_name. Failed items return agent_name, error, and task_id when the task row was persisted before the failure.",
			"group_id is useful for correlating tasks across the batch. It is omitted only when no task rows were persisted.",
			"depends_on between items in the same batch is not supported. Use individual send_task calls when dependencies are required.",
		},
	}
	cancelTaskHelpContract = helpToolContract{
		registryDescription: "Cancel a pending or blocked task. Accessible by the task sender pane, the sender agent, or trusted callers.",
		helpDescription:     "Cancel a sent task that is still pending or blocked.",
		notes: []string{
			"The sender pane, the sender agent, and trusted callers can call this tool.",
			"The response includes task_id and status only.",
		},
	}
)
