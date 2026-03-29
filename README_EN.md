# myT-x

**A terminal built on Windows, by a Windows user, for Windows users.**

- [Japanese README](./README.md)
- [English README](./README_EN.md)

## Disclaimer

The author assumes no responsibility for any damage caused by using this program.
Because of pane integration and related features, the application must be run with administrator privileges.

**Feature requests are always welcome.**

---

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
**Canvas Mode**
![Working image](sample6.png)

**I want to enjoy native Claude Code team development on Windows.**
**I want a good GUI. Nothing complicated. I want something visual and easy to understand.**
**I want to do a bit of everything.**
**Token budgets are limited, so I want a practical way to manage them well.**

---

## Key Features

| Feature | Description |
|---------|-------------|
| Terminal Splitting | Left/right and top/bottom pane splits, 5 layout presets |
| Agent Teams | Team coordination with Claude Code / Codex CLI / Gemini CLI |
| Auto Model Switch | Bulk or per-agent automatic model replacement for child agents |
| Git Worktree | Independent working directories per branch |
| 9 Viewers | File Tree / Git Graph / Diff / Input History / MCP Manager / Scheduler / Task Queue / Team Management / Error Log |
| Built-in MCP | Orchestration MCP + 200+ LSP-MCP integrations |
| Task Automation | Pane Scheduler (recurring) + Task Scheduler (sequential) |
| Canvas Mode | Visualize task flows between agents |
| Quake Mode | Instantly summon the window with a hotkey |
| Japanese IME | WebView2 process isolation + IME reset |

---

## Getting Started

Just double-click `myT-x.exe` to launch it.
On first launch, the configuration file is created automatically, so you can start using it immediately.

https://github.com/my-take-dev/myT-x/releases

---

## Documentation

See the `doc/` folder for detailed manuals.

| Document | Content |
|----------|---------|
| [Getting Started](doc/getting-started_en.md) | Installation, creating your first session |
| [Screen Layout](doc/screen-layout_en.md) | Menu bar, sidebar, main area, Activity Strip UI elements |
| [Terminal Operations](doc/terminal-operations_en.md) | Splitting, copy/paste, search, Quake Mode, sync input, chat bar |
| [Viewer System](doc/viewer-system_en.md) | Detailed guide for all 9 viewers (File Tree / Git Graph / Diff, etc.) |
| [Settings](doc/settings_en.md) | 6 settings tabs (Shell / Key Bindings / Worktree / Agent Model / Env Vars) |
| [Agent Teams](doc/agent-teams_en.md) | Team creation, member management, Orchestration MCP, Canvas Mode |
| [Task Scheduler](doc/task-scheduler_en.md) | Pane Scheduler and Task Scheduler usage |
| [Shortcuts](doc/shortcuts_en.md) | Complete keyboard shortcut reference |
| [Troubleshooting](doc/troubleshooting_en.md) | Common problems and solutions |

---

## About This Application

I expect to use this application for company work, so it is being continuously improved with OSS checks, security reviews, and ongoing AI-assisted refactoring to keep it safe and reliable.

I do not plan to introduce breaking changes to the core feature set. Internally, however, aggressive refactoring happens continuously, so if a feature breaks, fixes are applied as soon as the issue is found.

---

## How to Build

This project is operated with a `Makefile`.

```sh
# Development mode
make dev

# Production build
make build
```

---

## Supported Environment

Windows 10 / 11
