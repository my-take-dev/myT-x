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

**Claude Codeエージェントチーム**
![作業イメージ画像](sample.png)
**File View Mode**
md,openAPI(swagger),drawio...
![作業イメージ画像](sample2.png)  
![作業イメージ画像](sample2-2.png)  
**Editor**
![作業イメージ画像](sample3.png)
**ダッシュボード**
![作業イメージ画像](sample4.png)
**よくある指示簡易登録機能**
![作業イメージ画像](sample5.png)
**オーケストレーターMCP&キャンバスモード＆タイムライン**
![作業イメージ画像](sample6.png)

**I want to enjoy native Claude Code team development on Windows.**
**I want a good GUI. Nothing complicated. I want something visual and easy to understand.**
**I want to do a bit of everything.**
**Token budgets are limited, so I want a practical way to manage them well.**

---

## Key Features

| Feature | Description |
|---------|-------------|
| Terminal Splitting | Left/right and top/bottom pane splits, 5 layout presets |
| AutoStart | Launch commands into a new pane directly from the pane toolbar (up to 50 entries) |
| Agent Teams | Team coordination with Claude Code / Codex CLI / Gemini CLI |
| Auto Model Switch | Bulk or per-agent automatic model replacement for child agents |
| Git Worktree | Independent working directories per branch (setup scripts + progress indicator) |
| File View | Unified preview for Markdown / Mermaid / Swagger(OpenAPI) / draw.io / SQLite / Markmap / Vega / Vega-Lite / WaveDrom (`Ctrl+Shift+E`) |
| SQLite Viewer | Read-only table list + row data + CSV export for `.db` / `.sqlite` / `.sqlite3` |
| Prompt Presets | Register reusable prompt templates and append to chat input (global / project scopes, up to 200, `Ctrl+Shift+P`) |
| Usage Dashboard | Visualize Claude Code / Codex usage stats (agents / skills / slash commands, category tabs + daily bar charts, 30-day activity, `Ctrl+Shift+U`) |
| 13 Viewers | Editor / File View / Git Graph / Diff / Input History / MCP Manager / Scheduler / Task Queue / Single Task Runner / Team Management / Usage Dashboard / Prompt Presets / Error Log |
| Built-in MCP | Orchestration MCP + Single Task Runner MCP + 200+ LSP-MCP integrations |
| Task Automation | Pane Scheduler (recurring) + Task Scheduler (sequential) + Single Task Runner (lightweight, `Ctrl+Shift+J`) |
| Per-pane Chat Bar | Chat input bar docked at the bottom of each pane, click to target that pane |
| Command Palette | `Ctrl+P` for session switching, viewer launch, command execution (Menu Bar trigger button supported) |
| Canvas Mode | Visualize task flows between agents (with manual root pane selection) |
| Terminal Search | Incremental search, result counter, match highlighting (`Ctrl+F`) |
| Quake Mode | Instantly summon the window with a hotkey |
| Diff Inline Comments | Hover a diff line → `+` button → inline textarea → send as Markdown to an AI pane in bulk (`Ctrl+Shift+D`) |
| Markdown Outline | Collapsible outline panel in Markdown Preview (heading list + in-page jump) |
| Japanese IME | WebView2 process isolation + IME reset + auto-recovery on terminal focus |

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
| [Viewer System](doc/viewer-system_en.md) | Detailed guide for all 13 viewers (Editor / File View / Git Graph / Diff / Usage Dashboard / Prompt Presets, etc.) |
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
