# Agent Teams (AI Team Development)

The defining feature of myT-x is the ability to coordinate multiple AI agents as a team within a single application.

---

## Overview

With Agent Teams, you can:

- Launch different AI agents (Claude Code, Codex CLI, Gemini CLI) in each pane
- Automate task exchange between agents via MCP orchestration
- Save team definitions and restart with the same configuration anytime

---

## Creating an Agent Team Session

1. Click **+ New Session** in the left sidebar
2. Enter a session name and working directory
3. Check **Start as Agent Team**
4. Click **Create**

Sessions marked with the **A** badge in the sidebar are Agent Team sessions.

> Agent Teams require the tmux-shim to be installed.
> If not installed, "(shim not installed)" is displayed.

---

## Creating and Managing Teams

Open the **Orchestrator Teams** viewer (`Ctrl+Shift+T`) in the right sidebar.

### Team List

| Display | Content |
|---------|---------|
| Team name | Name of the team |
| Member count | Number of agents in the team |
| Reorder buttons | Drag or use arrow buttons to change order |

| Button | Description |
|--------|-------------|
| **+ New Team** | Create a new team |
| ✏ Edit | Modify team settings |
| 📋 Copy | Duplicate the team |
| 🗑 Delete | Delete the team (confirmation required) |
| ▶ Start Team | Launch the team |

### Team Editor

| Field | Description |
|-------|-------------|
| **Team name** | Team name (must be unique) |
| **Members list** | Agents belonging to the team |

#### Member Operations

| Button | Description |
|--------|-------------|
| **+ Add Member** | Add a new member |
| **+ Copy from another team** | Duplicate a member from an existing team |
| ✏ Edit | Modify member settings |
| 📋 Copy | Duplicate the member |
| 🗑 Delete | Remove the member |

### Member Editor

| Field | Description |
|-------|-------------|
| **Pane name** | Name displayed on the pane (unique within the team) |
| **Agent / Role** | The agent's role or specialization |
| **Model** | AI model override |
| **System prompt / Instructions** | Initial instructions for the agent |
| **Bootstrap delay** | Wait time in seconds before bootstrapping |

---

## Launching a Team

1. Click **▶ Start Team** in the team list
2. Review the member composition in the launch dialog
3. Click **Launch**

Each member is automatically assigned a pane, and configured instructions are sent as bootstrap messages.

---

## Orchestration MCP

### How It Works

myT-x includes a built-in Orchestration MCP.
AI agents in each pane can:

- Send tasks to other panes
- Receive tasks addressed to them
- View the list of team members

### Tool Reference

For the complete specification of all 18 tools (parameters, return values, permissions, validation constraints), see [MCP Orchestrator Tool Reference](mcp-orchestrator_en.md).

### Connecting External AI Tools

Check the MCP Manager (`Ctrl+Shift+M`) for connection details.

```
myT-x mcp stdio --session <session-name> --mcp <mcp-id>
```

You can also use the `MYTX_SESSION` environment variable instead of the `--session` flag.

### Supported AI Tools

| Tool | Connection Method |
|------|------------------|
| Claude Code | MCP stdio |
| Codex CLI | MCP stdio |
| Gemini CLI | MCP stdio |

---

## Quick Start Example

1. Create an Agent Team session
2. Split into 3 panes (%0, %1, %2)
3. Launch an AI in each pane
4. Enter the following in the first pane:

```
Please read `orchestrator.md`.
You are pane %0. Panes %1 and %2 already exist, so register yourself as the tester
and send a test message to receive a response.
```

> We recommend using lightweight models (Haiku or Codex Low) first to understand the workflow before switching to production models.

---

## Adding Members from the Pane Toolbar

Use the **👤+ Add Member** button in each pane's toolbar to add members to an existing team.

---

## Automatic Model Switching

In Agent Teams, a parent agent launches child agents.
The **Agent Model** tab in settings lets you automatically replace child agent models.

See [Settings > Agent Model](settings_en.md#tab-4-agent-model) for details.

---

## Canvas Mode

To visualize Agent Teams orchestration, use canvas mode.

Click the toggle button to display panes as nodes with task flows shown as connecting edges.

| Edge Display | Meaning |
|-------------|---------|
| Total tasks | Total number of tasks sent/received |
| pending | Unprocessed tasks |
| completed | Finished tasks |
| failed | Failed tasks |
| abandoned | Interrupted tasks |

---

## Team Storage Locations

| Location | Path | Purpose |
|----------|------|---------|
| Global | `%LOCALAPPDATA%\myT-x\teams\` | Team definitions usable across all projects |
| Project-local | `.myT-x\teams\` (in repository) | Project-specific team definitions |

---

## Next Steps

- [MCP Orchestrator Tool Reference](mcp-orchestrator_en.md) — Complete specification of all 18 tools
- [Task Scheduler](task-scheduler_en.md) — Automate task execution
- [Viewer System](viewer-system_en.md) — All viewers explained
- [Shortcuts](shortcuts_en.md)
