# Getting Started

## Installation

1. Download the latest `myT-x.exe` from the [Releases page](https://github.com/my-take-dev/myT-x/releases)
2. Place it in any folder
3. Double-click to launch

A configuration file (`config.yaml`) is created automatically on first launch, so you can start using it immediately.

---

## First Screen

After launching, you will see:

```
+------------------------------------------------------------+---+
|  [Settings] [A↻]  [Japanese|English]                        |   |
+------------+-----------------------------------------------+ A |
|            |                                               | c |
|  myT-x     |    "Create a session to get started"          | t |
| ---------- |                                               | i |
|            |        [▶ Quick Start]                         | v |
|            |                                               | i |
| [+ New]    |                                               | t |
+------------+-----------------------------------------------+ y |
|  Status bar                                                 |   |
+------------------------------------------------------------+---+
```

---

## Creating Your First Session

### Option 1: Quick Start (Fastest)

Click the **▶ Quick Start** button in the main area.
A session is created instantly in the default directory.

### Option 2: New Session

1. Click **+ New Session** in the left sidebar
2. Enter a **Session Name**
3. Select a **Working Directory** ("Select folder..." button)
4. Configure options as needed:
   - **Start as Agent Team** — Check for AI team development
   - **Use Claude Code environment variables** — Apply Claude Code env vars from settings
   - **Use additional pane environment variables** — Apply extra env vars
5. Click **Create**

> If you select a folder inside a Git repository, **Worktree options** appear automatically.
> You can create independent working directories per branch and work on multiple branches simultaneously.

---

## Basic Session Operations

| Action | How |
|--------|-----|
| Switch session | Click session name in left sidebar |
| Rename session | Double-click session name, or press F2 |
| Reorder sessions | Drag and drop |
| Close session | Right-click → "Close session" |
| Open in Explorer | Right-click → "Open in Explorer" |

### Session Types

Sessions are marked with badges in the left sidebar:

| Badge | Type | Description |
|-------|------|-------------|
| **S** | Standard | Regular terminal session |
| **A** | Agent Team | AI team development session |

When using Worktree, repository and branch names are also shown as badges.

---

## Next Steps

- [Screen Layout](screen-layout_en.md) — Understand each area of the UI
- [Terminal Operations](terminal-operations_en.md) — Learn pane splitting and shortcuts
- [Settings](settings_en.md) — Customize shell and key bindings
- [Agent Teams](agent-teams_en.md) — Start AI team development
