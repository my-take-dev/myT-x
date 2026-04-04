# Settings

Open the settings screen from the **Settings** button in the top-left corner. It has 6 tabs.

> Some settings require an application restart to take effect.

---

## Tab 1: General

### Shell

Choose the shell used in panes.

| Option | Description |
|--------|-------------|
| powershell.exe | Windows PowerShell (default) |
| pwsh | PowerShell 7+ |
| cmd | Command Prompt |
| bash | Git Bash, etc. |
| WSL | Windows Subsystem for Linux |

### Prefix Key

The trigger key combination for terminal actions.
Default is `Ctrl+b`, matching tmux conventions.

**Usage:** Press prefix key → "PREFIX" appears in status bar → Press action key

### Quake Mode

When ON, a global hotkey toggles window visibility.
Instantly summon the terminal while working in other applications.

### Global Hotkey

The keyboard shortcut for Quake Mode. Default: `Ctrl+Shift+F12`.

> If another application uses the same key combination, it won't work. Change to a different key.

### Default Session Directory

The directory used for Quick Start sessions.
Click "Select folder..." or enter a path directly.

---

## Tab 2: Key Bindings

Customize the action keys pressed after the prefix key.

| Action | Default | Description |
|--------|---------|-------------|
| Split Left/Right | `%` | Split pane horizontally |
| Split Top/Bottom | `"` | Split pane vertically |
| Toggle Zoom | `z` | Maximize / restore pane |
| Close Pane | `x` | Close the current pane |
| Detach | `d` | Disconnect (keep session alive) |

### Viewer Shortcuts

Customize keyboard shortcuts for each viewer:

| Viewer | Default |
|--------|---------|
| Editor | `Ctrl+Shift+W` |
| File Tree | `Ctrl+Shift+E` |
| Git Graph | `Ctrl+Shift+G` |
| Diff | `Ctrl+Shift+D` |
| Input History | `Ctrl+Shift+H` |
| MCP Manager | `Ctrl+Shift+M` |
| Pane Scheduler | `Ctrl+Shift+K` |
| Task Scheduler | `Ctrl+Shift+Q` |
| Teams | `Ctrl+Shift+T` |
| Error Log | `Ctrl+Shift+L` |

---

## Tab 3: Worktree (Git Integration)

Available when creating sessions inside Git repositories.

### Enable / Disable

When OFF, all Worktree-related actions are hidden.

### Force Cleanup

When ON, Worktrees can be deleted even with uncommitted changes.
When OFF, a confirmation dialog appears.

### Setup Scripts

Commands to run automatically after creating a Worktree.

**Examples:**
- `npm install` — Install Node.js dependencies
- `go mod download` — Download Go modules

Add multiple commands with the "+" button. They run in order.

### Copy Files

Files not tracked by Git to copy into new Worktrees.

**Example:** `.env`, `.env.local`

### Copy Directories

Directories to copy recursively.

**Example:** `.vscode`

---

## Tab 4: Agent Model

Automatically switch child agent models in Claude Code Agent Teams.

### Bulk Replacement

| Field | Description |
|-------|-------------|
| **From** | Source model name. Set to `ALL` to match any model |
| **To** | Replacement model name |

**Example:** Replace Opus with Sonnet to reduce cost

```
From: claude-opus-4-6
To:   claude-sonnet-4-5-20250929
```

### Per-Agent Overrides

Assign specific models to specific agents.

| Field | Description |
|-------|-------------|
| Agent name | Partial match, case-insensitive. Minimum 5 characters |
| Model | Model to use for matching agents |

- Rules are evaluated top to bottom; the first match wins
- Add with the "+ Add Override" button

**Example use cases:**

| Goal | Setup |
|------|-------|
| Lower overall cost | Bulk replace Opus → Sonnet |
| Keep security cautious | Override `security` to use Opus |
| Optimize per task | Add multiple overrides |

---

## Tab 5: Claude Code Environment Variables

Environment variables applied automatically when creating Claude Code sessions.

### Default Enabled

When ON, the "Use Claude Code environment variables" checkbox defaults to ON in the session creation dialog.

### Reasoning Level (CLAUDE_CODE_EFFORT_LEVEL)

| Level | Behavior |
|-------|----------|
| **high** | Thorough thinking for complex tasks |
| **medium** | Balanced for everyday work |
| **low** | Fast responses for simple tasks |

### Variable List

70+ official Claude Code environment variables are supported.

| Category | Key Variables |
|----------|--------------|
| API Auth | `ANTHROPIC_API_KEY`, `ANTHROPIC_AUTH_TOKEN` |
| Model Config | `ANTHROPIC_MODEL`, `ANTHROPIC_DEFAULT_SONNET_MODEL` |
| Thinking | `MAX_THINKING_TOKENS` |
| Features | `CLAUDE_CODE_DISABLE_AUTO_MEMORY`, `CLAUDE_CODE_DISABLE_BACKGROUND_TASKS` |
| MCP | `MCP_TIMEOUT`, `MCP_TOOL_TIMEOUT` |

Add variables with "+ Add Environment Variable"; remove with the ✕ button.

> System variables like `PATH` cannot be overridden.

---

## Tab 6: Pane Environment Variables

Additional environment variables applied to all panes.

### Default Enabled

When ON, the "Use pane environment variables" checkbox defaults to ON in the session creation dialog.

### Variable List

Add any environment variables you need.

**Examples:**
- `HTTP_PROXY` — Proxy settings
- `GOPATH` — Go workspace path

---

## Config File Location

Settings are saved in YAML format at:

```
%LOCALAPPDATA%\myT-x\config.yaml
```

The application starts even if the config file has parse errors.
Errors are logged, and default settings are used.

---

## Next Steps

- [Agent Teams](agent-teams_en.md) — AI team development setup and usage
- [Shortcuts](shortcuts_en.md) — Complete keyboard shortcut reference
- [Troubleshooting](troubleshooting_en.md) — Solutions to common problems
