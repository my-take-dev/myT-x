# MCP Orchestrator Tool Reference

Complete specification of all 18 tools provided by the myT-x built-in Orchestration MCP.
For an overview of Agent Teams, see [Agent Teams](agent-teams_en.md).

---

## Overview

The Agent Orchestrator MCP is a server that manages task communication between multiple AI agents.
AI agents in each pane can send/receive tasks, manage statuses, and add team members through MCP tools.

### Typical Workflow

1. `register_agent` — Register yourself as an agent (**required** prerequisite for all other tools)
2. `get_my_tasks` — Check pending tasks addressed to you
3. `get_task_message` — Retrieve the task message body using `send_message_id`
4. `acknowledge_task` — Record receipt acknowledgment if needed
5. Execute the task
6. `send_response` — Reply with `task_id` to complete the task

### Best Practices

- Always run `register_agent` first. Other tools require registration
- The `task_id` in `send_response` is required. Omitting it prevents task completion
- With `include_response_instructions=true` (default), `send_response` instructions are auto-appended
- Use `send_task` for direct agent-to-agent communication (no orchestrator intermediary required)
- Use `capture_pane` to check other agents' screens for progress or errors
- Use `add_member` to dynamically add members. Bootstrap messages are sent automatically
- After sending tasks with `depends_on`, call `activate_ready_tasks` once dependencies complete

---

## Task Status Values

| Status | Description |
|--------|-------------|
| `pending` | Ready to execute |
| `blocked` | Waiting for dependencies |
| `completed` | Done with response |
| `failed` | Delivery/execution error |
| `abandoned` | Not picked up (implicit timeout) |
| `cancelled` | Explicitly cancelled by sender |
| `expired` | TTL exceeded |

## Agent Status Values

| Status | Description |
|--------|-------------|
| `idle` | Waiting, accepts new tasks |
| `busy` | Working, no new tasks |
| `working` | Working, still accepts new tasks |

---

## Tool Reference

### 1. register_agent

Register an agent's pane ID and name, along with role and skills.
Re-calling with the same name updates the information.

**Access:** Anyone (no registration required)

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `name` | Yes | string | Agent name (alphanumeric + `._-`, max 64 chars) |
| `pane_id` | Yes | string | tmux pane ID (e.g., `%1`) |
| `role` | No | string | Role description (max 120 chars) |
| `skills` | No | array | Skills (max 20). Format: `[{"name":"Go","description":"..."}]` |

**Returns:** `name`, `pane_id`, `role`, `skills`, `pane_title`, (`warning`)

**Notes:**
- Required prerequisite for all other tools. Must be called first
- If another agent is already registered on the same `pane_id`, it is overwritten
- Registration/updates can be performed regardless of the caller's pane

---

### 2. list_agents

Get all agent information, cross-referenced with tmux list-panes.

**Access:** Registered agents only

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| (none) | — | — | — |

**Returns:** `registered_agents` (name, pane_id, role, skills, status), `unregistered_panes`, (`orchestrator`), (`warning`)

**Notes:**
- `registered_agents` includes status (`idle` / `busy` / `working` / `unknown`)

---

### 3. send_task

Send a task to a specific agent (unicast).

**Access:** Anyone can send (no orchestrator intermediary required)

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `agent_name` | Yes | string | Recipient agent name |
| `from_agent` | Yes | string | Your registered agent name (used for replies) |
| `message` | Yes | string | Task message (max 8000 chars) |
| `include_response_instructions` | No | boolean | Auto-append response template (default: `true`) |
| `expires_after_minutes` | No | integer | Task expiry (1-1440 minutes) |
| `depends_on` | No | array | Dependency task ID array (max 20) |

**Returns:** `task_id`, `agent_name`, `pane_id`, `sender_pane_id`, `sent_at`

**Notes:**
- Returns `task_id` on success. The recipient uses this for `send_response`
- Response instructions are auto-appended to the message by default
- When `depends_on` is specified, the task is created as `blocked` and activated via `activate_ready_tasks`
- Non-existent dependency IDs cause an error
- On delivery failure, the task becomes `failed`

---

### 4. send_tasks

Send tasks to multiple agents in batch, grouped by `group_id`.

**Access:** Registered agents only

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `from_agent` | Yes | string | Your registered agent name (used for replies) |
| `group_label` | No | string | Group label (max 120 chars) |
| `tasks` | Yes | array | Target array (1-10 items) |

Each element in `tasks`:

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `agent_name` | Yes | string | Recipient agent name |
| `message` | Yes | string | Task message (max 8000 chars) |
| `include_response_instructions` | No | boolean | Auto-append response template (default: `true`) |
| `expires_after_minutes` | No | integer | Task expiry (1-1440 minutes) |

**Returns:** `group_id`, `results` (agent_name + task_id or error), `summary` (sent, failed)

**Notes:**
- Success entries return `task_id` / `agent_name`; failure entries return `agent_name` / `error`
- `depends_on` between batch tasks is not supported. Use individual `send_task` for dependencies

---

### 5. get_my_tasks

Get tasks addressed to you (inbox).

**Access:** Registered agents only (caller name must match `agent_name`)

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `agent_name` | Yes | string | Your agent name |
| `status_filter` | No | string | `pending` / `blocked` / `completed` / `all` / `failed` / `abandoned` / `cancelled` / `expired` (default: `pending`) |

**Returns:** `agent_name`, `tasks` (task_id, status, sent_at, is_now_session, send_message_id, sender_pane_id, completed_at), `response_instructions`

**Notes:**
- Each task includes `send_message_id`. Pass it to `get_task_message` to retrieve the message body
- `response_instructions` (reply procedure) is included in the response

---

### 6. get_task_message

Retrieve task message body and metadata by `send_message_id`.

**Access:** Registered agents only (caller name must match `agent_name`)

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `agent_name` | Yes | string | Your agent name |
| `send_message_id` | Yes | string | Target send_message_id (`m-` prefix) |

**Returns:** `task_id`, `agent_name`, `send_message_id`, `status`, `sent_at`, `is_now_session`, `message` (content, created_at), (`sender_pane_id`), (`completed_at`)

**Notes:**
- Use `send_message_id` obtained from `get_my_tasks`
- Use this tool to read message bodies
- Use `get_task_detail` for progress, dependencies, and response content

---

### 7. get_task_detail

Get detailed status for a single task.

**Access:** Sender, assignee, or trusted callers

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `task_id` | Yes | string | Target task_id |

**Returns:** `task_id`, `status`, `agent_name`, (`completed_at`), (`acknowledged_at`), (`cancelled_at`), (`cancel_reason`), (`progress_pct`), (`progress_note`), (`progress_updated_at`), (`expires_at`), (`depends_on`), (`response` (content, created_at))

**Notes:**
- Completed tasks include `response.content` and `response.created_at`
- Does not include the message body. Use `get_task_message` (assignee) for that

---

### 8. acknowledge_task

Record receipt acknowledgment for a task (optional).

**Access:** Task assignee only

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `agent_name` | Yes | string | Your agent name |
| `task_id` | Yes | string | Task to acknowledge |

**Returns:** `task_id`, `agent_name`, `acknowledged_at`

**Notes:**
- Skipping has no impact on task processing
- Allows the sender to check `acknowledged_at` via `get_task_detail`

---

### 9. send_response

Reply to the task sender and mark the task as `completed`.

**Access:** Assignee of a pending task only

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `task_id` | Yes | string | Response target task_id |
| `message` | Yes | string | Reply message (max 8000 chars) |

**Returns:** `sent_to`, `sent_to_name`, (`warning`), (`task_id`, `task_status`, `completed_at`)

**Notes:**
- Omitting `task_id` causes an error. The task cannot be completed without it
- The message is sent to the sender's pane and the task becomes `completed`

---

### 10. update_status

Update your agent's work status.

**Access:** Self only

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `agent_name` | Yes | string | Your agent name |
| `status` | Yes | string | `idle` (waiting, accepts tasks) / `busy` (working, no new tasks) / `working` (working, accepts tasks) |
| `current_task_id` | No | string | Current task_id (empty string to clear) |
| `note` | No | string | Status note (max 200 chars) |

**Returns:** `agent_name`, `status`, `updated_at`

**Notes:**
- Other agents use `get_agent_status` to check this and select task recipients

---

### 11. get_agent_status

Get the latest status of a specific agent.

**Access:** Any registered agent

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `agent_name` | Yes | string | Target agent name |

**Returns:** `agent_name`, `status`, (`current_task_id`), (`note`), (`seconds_since_update`)

---

### 12. list_all_tasks

Get the status of all tasks in the system (team-wide monitoring).

**Access:** Any registered agent

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `status_filter` | No | string | `pending` / `blocked` / `completed` / `all` / `failed` / `abandoned` / `cancelled` / `expired` (default: `all`) |
| `assignee_name` | No | string | Filter by assignee agent |

**Returns:** `tasks` (task_id, agent_name, status, sent_at, is_now_session, send_message_id, sender_pane_id, completed_at), `summary` (pending, blocked, completed, failed, abandoned, cancelled, expired)

**Notes:**
- `get_my_tasks` is your personal inbox. `list_all_tasks` is the team-wide monitoring view

---

### 13. activate_ready_tasks

Evaluate blocked task dependencies and activate ready tasks to `pending`.

**Access:** Any registered agent

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `assignee_name` | No | string | Filter by assignee agent |

**Returns:** `activated` (task_id, agent_name), `still_blocked`

**Notes:**
- Call after `send_response` to activate tasks that depended on the completed task
- Blocked tasks with `cancelled` / `failed` / `abandoned` / `expired` / invalid dependencies are auto-cancelled

---

### 14. cancel_task

Cancel a `pending` or `blocked` task.

**Access:** Task sender only

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `task_id` | Yes | string | Task to cancel |
| `reason` | No | string | Cancellation reason (max 500 chars) |

**Returns:** `task_id`, `status`

---

### 15. update_task_progress

Update progress percentage or note for an assigned task.

**Access:** Task assignee only

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `task_id` | Yes | string | Target task_id |
| `progress_pct` | No | integer | Progress (0-100) |
| `progress_note` | No | string | Progress note (max 500 chars) |

**Returns:** `task_id`, `progress_updated_at`, (`progress_pct`)

**Notes:**
- At least one of `progress_pct` or `progress_note` is required

---

### 16. capture_pane

Get a pane's display content for another agent (screen capture).

**Access:** Any registered agent

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `agent_name` | Yes | string | Target agent name |
| `lines` | No | integer | Lines to capture (1-200, default: 50) |

**Returns:** `agent_name`, `pane_id`, `lines`, `content`, `warning`

**Notes:**
- Used for checking progress or errors on other agents' panes

---

### 17. add_member

Dynamically add a new team member: pane split, CLI launch, and bootstrap in one step.

**Access:** Registered agents only

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `pane_title` | Yes | string | Display name (max 30 chars) |
| `role` | Yes | string | Role description (max 120 chars) |
| `command` | Yes | string | CLI command (e.g., `claude`) (max 100 chars) |
| `args` | No | array | Command arguments (max 20 items, 200 chars each) |
| `custom_message` | No | string | Additional instructions (max 2000 chars) |
| `skills` | No | array | Skills array (same format as `register_agent`, max 20) |
| `team_name` | No | string | Team label (max 64 chars, default: `動的チーム`) |
| `split_from` | No | string | Pane ID to split from (default: caller's pane) |
| `split_direction` | No | string | `horizontal` / `vertical` (default: `horizontal`) |
| `bootstrap_delay_ms` | No | integer | Delay after CLI launch in ms (1000-30000, default: 3000) |

**Returns:** `pane_id`, `pane_title`, `agent_name`, (`warnings`)

**Notes:**
- Omitting `split_from` splits the caller's pane
- Claude CLI (claude, claude.exe, claude-code*) uses bracketed paste mode
- Title setting or bootstrap failures are returned as warnings; processing continues
- Added members automatically receive a bootstrap message with `register_agent` instructions

---

### 18. help

Get help on the orchestrator MCP system.

**Access:** Anyone (no registration required)

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| `topic` | No | string | Tool name for specific help. Omit for overview |

---

## Input Validation Constraints

| Constraint | Value |
|-----------|-------|
| Max agent name length | 64 chars |
| Agent name pattern | `^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$` |
| Max role length | 120 chars |
| Max skill name length | 100 chars |
| Max skill description length | 400 chars |
| Max skills per agent | 20 |
| Max message length | 8000 chars |
| Max capture lines | 200 |
| Max pane title length | 30 chars |
| Max command length | 100 chars |
| Max custom message length | 2000 chars |
| Max team name length | 64 chars |
| Max command args | 20 |
| Max arg length (each) | 200 chars |
| Max status note length | 200 chars |
| Max cancel reason length | 500 chars |
| Max progress note length | 500 chars |
| Max group label length | 120 chars |
| Max batch tasks | 10 |
| Max depends_on tasks | 20 |
| task_id format | `t-[A-Za-z0-9]+` |
| send_message_id format | `m-[A-Za-z0-9]+` |

---

## Next Steps

- [Agent Teams](agent-teams_en.md) — AI team development overview
- [Task Scheduler](task-scheduler_en.md) — Automate task execution
- [Viewer System](viewer-system_en.md) — All viewers including MCP Manager
