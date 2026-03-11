# MCP Agent Orchestrator - Operations Manual

A practical guide for registering agents, sending tasks, replying, and checking status between multiple conversational AIs running on tmux.

## Core rules

- Do not assume pane numbers are fixed. `pane_id` can change every time, so do not reuse old values such as `%0`, `%1`, or `%2`.
- Always check the current registration state before starting work. Do not begin `send_task` or `send_response` without checking `list_agents()`.
- If registration is empty or incomplete, confirm the registration approach with the user first.
- Do not start the actual task before registration is complete.
- **`list_agents()` can only be used by already registered agents.** If you are not registered yet, call `register_agent` for yourself first.

## Checks you must do first

1. Register yourself with `register_agent()` if you are not registered yet.
2. Run `list_agents()` and inspect the currently registered agents.
3. Confirm that both the sender agent name and the destination agent name are already registered.
4. If registration is missing, incorrect, or incomplete, stop and confirm the registration policy before continuing.

## Rules when registration is empty or incomplete

An empty registration state is normal. Always assume cases such as a DB reset, a restart, or a newly launched session.

In that case, ask the user which of the following approaches to use before proceeding:

1. The first conversational AI that received instructions checks the current `pane_id` to name mapping for all running AIs and performs `register_agent` for all of them.
2. Each conversational AI that is already running registers itself in its own execution context with `register_agent`.

Do not arbitrarily choose one approach without confirmation.

Points to confirm with the user:

- Whether registration should be done in bulk or by self-registration
- Which names should be used
- Which AI should be used as the sender

## Recommended flow

### 1. Register yourself

Because `list_agents()` can only be called by a registered agent, register yourself first if needed.

```text
register_agent(name="<self_name>", pane_id="<current_pane_id>")
```

### 2. Check current state

`list_agents()`

- Run this after registering yourself
- Check both the registered agents and any unregistered panes
- If the expected AI names are missing, move on to additional registration

### 3. Confirm the registration policy

If other agents are still missing, confirm the registration approach with the user.

Example confirmation:

`The current registration list is empty. Should I register all AIs from the first instructed agent, or should each AI register itself? I will continue after registration is complete.`

### 4. Register the required agents

Either registration method is fine, but always confirm the current `pane_id` to name mapping before registering.

Example:

```text
register_agent(name="<agent_name>", pane_id="<current_pane_id>", role="<role>", skills=["<skill1>", "<skill2>"])
```

Registration notes:

- Always use the current `pane_id`
- If you register a different name on the same `pane_id`, the existing registration is replaced
- `orchestrator` is a reserved name and cannot be overwritten from another pane

### 5. Verify again

After registration, run `list_agents()` once more and confirm that both the sender and the target agent for this operation are visible.

### 6. Send a task

```text
send_task(agent_name="<target_agent>", from_agent="<sender_agent>", message="<request>", task_label="<label>")
```

Notes when sending:

- `from_agent` is required because it is the reply destination
- `from_agent` must be a registered name
- Direct agent-to-agent communication is supported
- A reply template containing a specific `task_id` and `send_response(...)` example is automatically appended to the end of the message body

### 7. Check tasks on the receiving side

```text
get_my_tasks(agent_name="<self_agent_name>")
```

- The receiving agent checks tasks addressed to itself
- Identify the `task_id` and use it when replying
- The received body also contains `task_id=<...>` and `send_response(task_id="...", message="...")`, so in normal cases the body alone is enough to reply

### 8. Send a response

```text
send_response(task_id="<task_id>", message="<reply>")
```

- The receiver who got the task must send the reply directly
- Do not have a third party complete the task on someone else's behalf
- `send_response` records both the reply and task completion
- Use the `task_id=<...>` value shown in the received message as-is

### 9. Check status

```text
check_tasks(status_filter="all")
```

- Confirm the transition from `pending` to `completed`
- Even when testing long replies, always verify status through this step

## Tool-by-tool usage

### `register_agent`

Purpose:

- Link an agent name to the current `pane_id`
- Re-register after restart
- Correct a bad registration

Basic form:

```text
register_agent(name="<agent_name>", pane_id="<current_pane_id>")
```

Notes:

- Add `role` and `skills` only when needed
- Any agent can register or update

### `list_agents`

Purpose:

- Check the current situation before starting work
- Detect missing registrations
- Verify state after re-registration

Operational notes:

- Only registered agents can run it. If you are unregistered, call `register_agent` first
- Always use it first after registering yourself
- If you are unsure, run it again

### `send_task`

Purpose:

- Send work requests
- Send reminders
- Consult another agent

Notes:

- Reply instructions are appended automatically by default (`include_response_instructions=false` disables this)
- The auto-appended template includes a concrete `task_id` and `send_response(task_id="...", message="...")`
- `from_agent` is the reply destination itself
- `task_label` is an optional management label with a maximum of 120 characters. If omitted, the first 50 characters of the message are used automatically

### `get_my_tasks`

Purpose:

- Check tasks assigned to yourself
- Retrieve `task_id` again if needed

Notes:

- Intended to be used by the receiving agent itself
- Because `task_id` is also shown in the received body, replies can usually be sent even without calling `get_my_tasks`
- Even if there are questions about ACL behavior, operationally this should still be treated as receiver-only

### `send_response`

Purpose:

- Reply to the requesting side
- Record task completion

Notes:

- The message body can be short or long
- At minimum, short replies, roughly 500-character replies, and roughly 1000-character replies have already been verified in operations

### `check_tasks`

Purpose:

- Monitor all tasks
- Check task status (`pending` / `completed` / `failed` / `abandoned`)
- Investigate cases where no response was received

Notes:

- `status_filter` can be used for filtering (optional, default: `"all"`). Valid values: `"all"` / `"pending"` / `"completed"` / `"failed"` / `"abandoned"`
- Use `agent_name` to retrieve tasks for a specific agent only

### `capture_pane`

Purpose:

- Check the state of another pane

Notes:

- In the current tmux shim, it may return only a `warning` without actual pane content
- Even if it does not work, evaluate that separately from whether task communication itself is working

## Operational cautions

- Do not copy and paste old pane numbers from previous notes or documents
- If unregistered, start by calling `register_agent` for yourself and then use `list_agents()`
- If registration is empty, confirm the registration method with the user before proceeding
- Before sending anything, make sure both the sender and the destination are registered
- Replies must always be sent by the actual recipient using `send_response`
- For connectivity checks, do not stop after `send_task` succeeds; confirm that `check_tasks()` reaches `completed`

## Minimum execution order

1. If you are not registered, register yourself with `register_agent(...)`
2. `list_agents()`
3. If other agents are missing, confirm the registration policy with the user
4. Run the necessary `register_agent(...)` calls
5. Re-check with `list_agents()`
6. `send_task(...)`
7. On the receiving side, run `get_my_tasks(...)`
8. On the receiving side, run `send_response(...)`
9. `check_tasks(status_filter="all")`

As long as you keep this order, operational mistakes become much less likely.
