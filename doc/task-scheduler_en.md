# Task Scheduler

myT-x includes two types of schedulers for automating repetitive work and sequential tasks.

---

## Pane Scheduler (Recurring Execution)

**Shortcut:** `Ctrl+Shift+K`

Sends messages to a pane at regular intervals.

### Use Cases

- Send periodic instructions to AI agents
- Execute commands at fixed intervals
- Automate monitoring tasks

### Creating a Scheduler

1. Open the **Scheduler** icon in the Activity Strip (`Ctrl+Shift+K`)
2. Click **+ New Scheduler**
3. Configure:

| Field | Description | Example |
|-------|-------------|---------|
| Title | Scheduler name | "Periodic review" |
| Target pane | Pane to send to | %0 |
| Message | Content to send | "Report your progress" |
| Interval (seconds) | Time between sends | 300 (5 minutes) |
| Max count | 0 for unlimited | 10 |

4. Click **Start**

### Scheduler States

| State | Display | Action |
|-------|---------|--------|
| Running | running | Stop button to pause |
| Stopped | stopped | Resume button to restart |
| Completed | completed | Max count reached |

---

## Task Scheduler (Sequential Execution)

**Shortcut:** `Ctrl+Shift+Q`

Queue tasks and execute them one by one in order.

### Use Cases

- Execute multiple tasks in sequence
- Stage-by-stage instructions for AI agents
- Build → Test → Deploy pipelines

### Scheduler Settings (Persistent Configuration)

Open the settings screen from the **Settings** button on the left side of the toolbar. Settings are saved to `config.yaml` and persist across restarts.

| Field | Description | Default |
|-------|-------------|---------|
| Wait after /new (seconds) | Seconds to wait after sending /new during Pre-Execution (0–60) | 10 |
| Idle wait timeout (seconds) | Timeout for idle detection (10–600) | 120 |
| Target panes | `task_panes` (only task-assigned panes) / `all_panes` (all session panes) | task_panes |

#### Message Templates

Save frequently used messages with a name and select them when creating tasks.

| Action | How |
|--------|-----|
| Add template | Click "+ Add template" in settings → enter name and body |
| Use template | Select from the dropdown in the task form → appended to the message |
| Edit / Delete | Manage from the template list in settings |

### Adding Tasks

1. Open the **Task Scheduler** icon in the Activity Strip (`Ctrl+Shift+Q`)
2. Click **+ New Task**
3. Configure:

| Field | Description | Example |
|-------|-------------|---------|
| Title | Task name | "Run tests" |
| Message | Command to send | `go test ./...` |
| Target pane | Destination pane | %0 |
| Clear before running | Clear pane first | ON |
| Clear command | Command to clear | `clear` |

4. Click **Save** to add to queue

### Queue Operations

| Button | Description |
|--------|-------------|
| ▶ **Start Queue** | Begin sequential execution |
| ⏸ **Pause** | Pause after the current task completes |
| ▶ **Resume** | Resume a paused queue |
| ⏹ **Stop** | Stop completely; remaining tasks are skipped |

### Task States

| State | Meaning |
|-------|---------|
| **pending** | Waiting (not yet executed) |
| **completed** | Finished successfully |
| **failed** | Execution failed |
| **abandoned** | Interrupted by queue stop |

### Templates

Save and load frequently used task combinations as templates.

| Action | How |
|--------|-----|
| Save | Save current tasks as a template |
| Load | Select a template to restore |
| Delete | Remove a template |

---

## Choosing Between the Two Schedulers

| | Pane Scheduler | Task Scheduler |
|---|---|---|
| **Pattern** | Same message repeated | Different tasks in sequence |
| **Use case** | Periodic execution, monitoring | Pipelines, staged instructions |
| **Completion** | Count limit or manual stop | All tasks done or manual stop |
| **Complexity** | Simple | Detailed per-task settings |

---

## Next Steps

- [Agent Teams](agent-teams_en.md) — AI team development and coordination
- [Viewer System](viewer-system_en.md) — All viewers explained
