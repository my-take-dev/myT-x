# myT-x

**A terminal built on Windows, by a Windows user, for Windows users.**

- [Japanese README](./README.md)
- [English README](./README_EN.md)

## Disclaimer

The author assumes no responsibility for any damage caused by using this program.
Because of pane integration and related features, the application must be run with administrator privileges.

**Feature requests are always welcome.**

## How to verify it works

This project is operated with a `Makefile`.

```sh
# Development mode
make dev

# Production build
make build
```

## Overview

**Agent Team**
![Working image](sample.png)
**Directory View**
![Working image](sample2.png)
**Git Graph**
![Working image](sample3.png)
**Diff View**
![Working image](sample4.png)
**Start a New Session**
![Working image](sample5.png)

**I want to enjoy native Claude Code team development on Windows.**  
**I want a good GUI. Nothing complicated. I want something visual and easy to understand.**  
**I want to do a bit of everything.**  
**Token budgets are limited, so I want a practical way to manage them well.**

**Main features:**

- Terminal pane splitting and layout management
- Automatic model switching for Claude Code Agent Teams
- Git Worktree integration
- **Viewer System** with four viewers: File Tree / Git Graph / Diff View / Error Log
- Centralized management of Claude Code environment variables
- Fast pane data transfer over WebSocket
- Built-in MCP support
- 200+ LSP-MCP integrations, including extensions
- Orchestration MCP

## Orchestration MCP

[Usage details](./orchestrator.md)

- You can handle orchestration from this single app.
- Connection details are available from the MCP management panel in the right sidebar.
- Humans are responsible for launching the conversational AIs in each pane.
- Orchestration can coordinate `claude code`, `codex cli`, and `gemini cli`.
- Roles for each AI can be assigned manually or generated automatically by another AI.

### Example prompt to get started

```text
Please read `orchestrator.md`.
You are pane %0. Panes %1 and %2 already exist, so register yourself as the tester
and send a test message to receive a response.
```

If you make the AI read the instructions properly, it usually connects successfully.
Use lighter models such as Haiku or Codex Low first to understand how startup and coordination work.

## About this application

I expect to use this application for company work, so it is being continuously improved with OSS checks, security reviews, and ongoing AI-assisted refactoring to keep it safe and reliable.

I do not plan to introduce breaking changes to the core feature set. Internally, however, aggressive refactoring happens continuously, so if a feature breaks, fixes are applied as soon as the issue is found.

---

## Getting started

Just double-click `myT-x.exe` to launch it.  
On first launch, the configuration file is created automatically, so you can start using it immediately.

https://github.com/my-take-dev/myT-x/releases

---

## Automatically switch models for Agent Teams

In Claude Code Agent Teams, a parent agent launches child agents one after another.  
myT-x can automatically replace the model used by those child agents at launch time.

Open the **Agent Model** tab in the settings screen. Two mechanisms are available.

### Bulk replacement for all child agents

If you enter model names in **From** and **To**, every child agent will have its model replaced automatically when it starts.

```text
Settings input:

  From:  claude-opus-4-6
  To:    claude-sonnet-4-5-20250929
```

With this setup, all child agents will run on Sonnet instead of Opus.  
This is useful when you want to reduce cost.

> `ALL` wildcard: If you set `From` to `ALL`, every child agent will be replaced with the target model regardless of its original model name.

### Per-agent overrides

If there are specific agents that should use a stronger model, add overrides.

```text
Settings input:

  #1  Agent name: security   Model: claude-opus-4-6
  #2  Agent name: reviewer   Model: claude-sonnet-4-5-20250929
  #3  Agent name: coder      Model: claude-sonnet-4-5-20250929
```

Agent names are matched by partial string match and are case-insensitive.  
Rules are evaluated from top to bottom, and the first match wins.

**Example use cases:**

| Goal | Setup |
|------|-------|
| Lower overall cost | Replace Opus with Sonnet |
| Keep security work more cautious | Override `security` to use Opus |
| Optimize by task type | Add multiple overrides for different agent names |

---

## Set reasoning level globally

The **Environment Variables** tab includes a dedicated selector for reasoning level.

```text
Settings input:

  Reasoning Level (CLAUDE_CODE_EFFORT_LEVEL): [low / medium / high]
```

The selected value is applied automatically to every pane.  
You do not need to `export` it manually whenever you open a new pane.

| Reasoning level | Description |
|-----------------|-------------|
| **high** | More deliberate thinking for complex tasks or important decisions |
| **medium** | Balanced mode for everyday work |
| **low** | Fast responses for simple tasks and chat |

### Claude Code environment variables

The **Environment Variables** tab also includes a dedicated section for Claude Code variables.  
Variables configured there are applied automatically when creating Claude Code sessions.  
You can enable or disable them with the **Use Claude Code environment variables** checkbox in the session creation dialog.

### Other environment variables for panes

On the same screen, you can add any environment variables you want.  
Use the **+ Add environment variable** button to enter a name and value.  
These variables are also applied automatically to all panes.  
Their usage can be controlled with the **Use pane environment variables** checkbox when creating a session.

---

## Creating an Agent Team session

Use **+ New Session** in the sidebar, then choose a folder to create a session.  
If you enable **Start as Agent Team**, that session will be treated as an Agent Team session.

In the sidebar, normal sessions are marked with `S` and Agent Team sessions are marked with `A`.

---

## Understanding the screen layout

```text
+------------------------------------------------------------+---+
|  [Settings]                                                |   |
+------------+-----------------------------------------------+ A |
|            |  ┌──────────────┬──────────────┐              | c |
|  myT-x     |  │              │              │              | t |
|            |  │   Pane 1     │   Pane 2     │              | i |
| ---------- |  │              │              │              | v |
|            |  │  PowerShell  │  PowerShell  │              | i |
| S Work A   |  └──────────────┴──────────────┘              | t |
| S Work B   |                                               | y |
| A Agent    |                                               |   |
|            |                                               | S |
| [+ New]    |                                               | t |
+------------+-----------------------------------------------+ r |
|  Work A | Panes: 2 | 13:42             [SYNC] [PREFIX]     | i |
+------------------------------------------------------------+ p |
                                                              +---+
```

**Left sidebar**

- Session list. Click to switch, drag to reorder, double-click to rename.

**Main area**

- Terminal panes. Split and arrange them freely.

**Activity Strip on the far right**

- Contains icons for switching viewers: File Tree / Git Graph / Diff View / Error Log.
- If there are unread items in Error Log, a badge (`●`) appears.

**Bottom status bar**

- Shows the current session name and state.

---

## Useful terminal operations

### Split panes

Use the pane toolbar icons to split horizontally or vertically.  
When you have many panes, the layout buttons let you reorganize them instantly with equal horizontal, equal vertical, left-main, top-main, or tiled layouts.

### Quick summon (Quake Mode)

Press `Ctrl+Shift+F12` to show or hide the window.  
This lets you call up the terminal only when you need it while working in a browser or editor.

### Quick search

Press `Ctrl+P` to fuzzy-search session names and branch names.

### Copy and paste

Selected text is copied automatically. Right-click pastes it. `Ctrl+C` and `Ctrl+V` also work naturally.

### Change font size

Use `Ctrl` + mouse wheel to zoom in or out.

### File drop

Drop a file onto a pane to paste its path.

### Text search

Press `Ctrl+F` to search pane output. This is useful for finding errors in long logs.

---

## Viewer System (developer tools)

Click the icons in the **Activity Strip** on the far right to open each viewer.  
You can also toggle them with keyboard shortcuts. Press `Escape` to close the active viewer.

### File Tree

Shows files and folders in the session working directory using a virtualized tree.  
Selecting a file displays a preview, and you can also copy its path.

| Shortcut | `Ctrl+Shift+E` |
|----------|----------------|

### Git Graph

Displays commit history as an SVG graph.  
You can quickly understand branch splits and merges, and selecting a commit lets you inspect its diff.

| Shortcut | `Ctrl+Shift+G` |
|----------|----------------|

### Diff View

Displays the result of `git diff HEAD` using a file tree plus line-by-line diffs.  
It includes staged changes, unstaged changes, and newly added files.  
Collapsed unchanged sections can be expanded with **Expand hidden lines**.

| Shortcut | `Ctrl+Shift+D` |
|----------|----------------|

### Error Log

Displays application logs at Warn and Error level in real time.  
Logs are persisted in JSONL format, so you can review them even after the session ends.  
If unread errors exist, the Activity Strip icon shows a badge.

| Shortcut | `Ctrl+Shift+L` |
|----------|----------------|

---

## Customize in settings

Use the **Settings** button in the top-left corner to open six configuration tabs.

### General

| You can change | Result |
|----------------|--------|
| Shell | Changes the shell used in panes (`PowerShell` / `pwsh` / `cmd` / `bash` / `WSL`) |
| Prefix | Changes the shortcut prefix key (default: `Ctrl+b`) |
| Quake Mode | Enables hotkey-based show/hide behavior |
| Global Hotkey | Changes the Quake Mode shortcut (default: `Ctrl+Shift+F12`) |

### Key bindings

You can customize the keys used after the prefix key.

| Action | Default |
|--------|---------|
| Split left/right | `%` |
| Split top/bottom | `"` |
| Toggle zoom | `z` |
| Close pane | `x` |
| Detach | `d` |

### Agent Model / Environment Variables / Claude Code Environment Variables

See the sections above: **Automatically switch models for Agent Teams**, **Set reasoning level globally**, and **Claude Code environment variables**.

### Worktree (Git integration)

If you create a session inside a Git repository, Worktree features become available.  
Dedicated working directories are created automatically for each branch, and the app also helps with cleanup.

| You can change | Result |
|----------------|--------|
| Enable / Disable | Hides Worktree-related actions when turned off |
| Force delete | Allows deleting a Worktree even when uncommitted changes exist |
| Setup script | Runs commands such as `npm install` automatically after creation |
| Copy files | Automatically copies files not tracked by Git, such as `.env` |

In the sidebar, repository names and branch names are shown as badges, making relationships like `main -> feature/xxx` easy to understand at a glance.

---

## Shortcut list

### Terminal operations

| Key | Action |
|-----|--------|
| `Ctrl+b` -> `%` | Split left/right |
| `Ctrl+b` -> `"` | Split top/bottom |
| `Ctrl+b` -> `z` | Zoom (maximize / restore) |
| `Ctrl+b` -> `x` | Close pane |
| `Ctrl+b` -> `d` | Detach |
| `Ctrl+P` | Quick search |
| `Ctrl+F` | Text search |
| `Ctrl+C` | Copy if text is selected, otherwise send interrupt |
| `Ctrl+V` | Paste |
| `Ctrl+Wheel` | Change font size |
| `Ctrl+Shift+F12` | Show / hide window |

### Viewer System

| Key | Action |
|-----|--------|
| `Ctrl+Shift+E` | Show / hide File Tree |
| `Ctrl+Shift+G` | Show / hide Git Graph |
| `Ctrl+Shift+D` | Show / hide Diff View |
| `Ctrl+Shift+L` | Show / hide Error Log |
| `Escape` | Close the active viewer |

---

## Troubleshooting

**The settings look broken**  
The app will still start even if the settings file has problems. Fix it from the settings screen.

**Global hotkey does not work**  
Check whether another application is already using the same shortcut. You can change it in settings.

**Pane splitting does not work**  
Press `Ctrl+b` first, confirm that `PREFIX` appears in the status bar, and then press the next key.

---

## Supported environment

Windows 10 / 11
