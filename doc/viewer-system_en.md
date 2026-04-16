# Viewer System

Thirteen development tools accessible from the **Activity Strip** on the right edge.
Click an icon or use a keyboard shortcut to toggle. Press `Escape` to close.

## Display Modes

Toggle with the button at the bottom of the Activity Strip:

| Mode | Description |
|------|-------------|
| **Overlay** | Displayed on top of the main area; terminal panes are hidden |
| **Docked** | Displayed alongside the main area; drag the divider to resize |

---

## 1. Editor

**Shortcut:** `Ctrl+Shift+O` (changed from `Ctrl+Shift+W` in v1.0.4)

A Monaco Editor-based code editor with a side-by-side file tree and editing area.

### Left Panel (File Tree / Search)

| Action | How |
|--------|-----|
| Expand/collapse folder | Click the folder icon |
| Select file | Click the file name — opens in the right panel for editing |
| Search files | Click the 🔍 button in the header to switch to search mode |

#### Header Buttons

| Button | Description |
|--------|-------------|
| 🔍 | Switch to file search mode |
| +F | Create a new file |
| +D | Create a new folder |
| ↻ | Reload file tree |

#### Context Menu (Right-click)

| Item | Description |
|------|-------------|
| Copy Path / Copy Relative Path | Copy the path to clipboard |
| New File / New Folder | Create under the selected directory |
| Rename | Rename the file or folder |
| Delete | Delete (confirmation required) |

### Right Panel (Monaco Editor)

| Element | Description |
|---------|-------------|
| File name + `*` | `*` indicates unsaved changes |
| Language label | Language detected from file extension |
| **Save** button | Save changes (`Ctrl+S` also works) |

**Notes:**
- Files larger than 1 MB are partially loaded; a warning appears in the toolbar
- `Ctrl+F` opens Monaco's built-in search; use the 🔍 header button for file search

---

## 2. File View

**Shortcut:** `Ctrl+Shift+E`

Displays document files in the session's working directory as a tree (renamed from `file-tree` to `file-view` in v1.0.6).
Optimized for documentation authoring and spec review — only files with target extensions appear in the tree.

### Filtered Extensions

| Extension | Preview |
|-----------|---------|
| `.md` | Markdown rendering with relative-path image support |
| `.mmd` | Mermaid diagrams |
| `.drawio` / `.drawio.svg` / `.drawio.xml` | draw.io diagrams |
| `.yaml` / `.yml` / `.json` | Swagger / OpenAPI auto-detection (Swagger UI launched only when `openapi:` / `swagger:` is detected in the first 1KB; otherwise raw text) |
| `.db` / `.sqlite` / `.sqlite3` | SQLite Viewer (table list + row data + CSV export) |

Folders that contain no matching descendants are hidden recursively.

### Left Panel (Tree / Search)

| Action | How |
|--------|-----|
| Expand/collapse folder | Click the folder icon |
| Select file | Click the file name — preview appears in the right panel |
| Search files | `Ctrl+F` to switch to search mode; search by name or path |

### Right Panel (File Preview)

| Kind | Display |
|------|---------|
| Markdown | `react-markdown` + GFM. Relative `<img>` resolved via `DevPanelReadBinary` to a blob URL |
| Mermaid | `mermaid@11` (lazy load) |
| Swagger / OpenAPI | `swagger-ui-react@5` (YAML/JSON, lazy load) |
| draw.io | `.drawio.svg` shown as `<img>`; `.drawio` / `.drawio.xml` shown as XML preview |
| SQLite | Table list + column info + virtualized row data + paging + CSV export |
| Other | Raw text (line selection / copy supported) |

### Header Buttons

| Button | Description |
|--------|-------------|
| Raw / Preview toggle | Switch between rendered and raw text |
| ↻ | Reload file tree |
| ✕ | Close viewer |

### SQLite Viewer

Open `.db` / `.sqlite` / `.sqlite3` files in read-only mode for browsing.

| Element | Description |
|---------|-------------|
| Left pane | Table list (click to switch) |
| Right pane (top) | Column info (type, NULL allowed, primary key) |
| Right pane (center) | Row data (virtualized; NULL is rendered distinctly from empty string) |
| Paging | Previous / Next |
| **CSV Export** | Save the current table as a CSV file |

The connection is read-only (`mode=ro&_pragma=busy_timeout(5000)`) and path-jailed; table names are double-quote-escaped to prevent SQL injection.

---

## 3. Git Graph

**Shortcut:** `Ctrl+Shift+G`

Displays commit history as a visual SVG graph.

### Header

| Element | Description |
|---------|-------------|
| **All Branches** checkbox | Shows all branches including remotes when checked |
| ↻ | Reload |
| ✕ | Close |

### Branch Status Bar

Shows the current branch name and tracking information.

### Left Panel (Commit Graph)

- Branch splits and merges are shown as connecting lines
- Click a commit to see details in the right panel
- "Load more..." at the bottom to fetch older commits

### Right Panel (Commit Details)

| Display | Content |
|---------|---------|
| Commit hash | Click to copy |
| Author | Commit author name |
| Message | Commit description |
| Diff | Changed files with line-by-line differences |

---

## 4. Diff

**Shortcut:** `Ctrl+Shift+D`

Displays `git diff HEAD` visually. Staging operations are also available here.

### Header

| Element | Description |
|---------|-------------|
| **Tree / Flat** toggle | Switch display mode |
| ↻ | Reload |
| ✕ | Close |

### Statistics Bar

Summary of changes: `+added` `-deleted` `Files: count`

### Tree Mode

**Left Panel (File Tree)**
- Shows changed files in a directory structure
- Click a file to view its diff

**Right Panel (Commit Panel)**
- Branch information
- **Commit message** text area
- Git operation buttons:

| Button | Description |
|--------|-------------|
| **Pull** | Fetch and merge from remote |
| **Fetch** | Fetch from remote (no merge) |
| **Commit** | Commit staged changes |
| **Commit & Push** | Commit and push to remote |
| **Push** | Push committed changes to remote |

### Flat Mode (Staging List)

All changed files in a flat list with staging controls:

| Action | How |
|--------|-----|
| Stage file | Click the + button on the file row |
| Unstage file | Click the - button |
| Discard changes | Click the discard button (confirmation required) |
| Stage all | **Stage All** button |
| Unstage all | **Unstage All** button |

### Diff Display

| Display | Meaning |
|---------|---------|
| Green background | Added lines |
| Red background | Deleted lines |
| Gray background | Unchanged context lines |
| "Expand hidden lines" | Expand collapsed sections |

---

## 5. Input History

**Shortcut:** `Ctrl+Shift+H`

Shows a log of commands and messages sent to panes.

| Display | Content |
|---------|---------|
| Timestamp | When the input was sent |
| Pane ID | Which pane received it |
| Input text | The content that was sent |
| Source | Origin (keyboard, chat, sync input) |

### Header Buttons

| Button | Description |
|--------|-------------|
| Copy | Copy all entries to clipboard |
| Mark read | Clear all unread badges |
| ↻ | Reload |
| ✕ | Close |

---

## 6. MCP Manager

**Shortcut:** `Ctrl+Shift+M`

Manage MCP (Model Context Protocol) servers available in the session.

### Categories

| Category | Content |
|----------|---------|
| **Agent Orchestrator** | Orchestration MCPs for inter-agent communication |
| **Single Task Runner** | Single Task Runner MCPs for sequential task execution on a single pane |
| **LSP-MCP** | Language Server Protocol MCPs; 200+ language extensions |

### Operations

- Click a category to switch
- View connection details for each MCP
- Toggle MCP servers on/off

### External Connections

The MCP Manager shows connection info for external AI tools (Claude Code, Codex CLI, Gemini CLI).
Usage of the `--session` flag is documented here.

### Related Documentation

- [MCP Orchestrator Tool Reference](mcp-orchestrator_en.md) — Complete specification of all 19 tools

---

## 7. Pane Scheduler (Schedule)

**Shortcut:** `Ctrl+Shift+K`

Sends messages to a pane at regular intervals.

### Scheduler List

| Display | Content |
|---------|---------|
| Scheduler name | Task name |
| Status | running / stopped / completed |
| Execution count | Number of times executed |
| Next fire | Time until next execution |

| Button | Description |
|--------|-------------|
| ✏ Edit | Modify scheduler settings |
| 🗑 Delete | Remove the scheduler |
| ▶ / ⏸ | Start / Stop |
| **+ New** | Create a new scheduler |

### Scheduler Settings

| Field | Description |
|-------|-------------|
| Title | Scheduler name |
| Target pane | Pane to send messages to |
| Message | Content to send |
| Interval (seconds) | Time between sends |
| Max count | 0 for unlimited |

---

## 8. Task Scheduler (Task Queue)

**Shortcut:** `Ctrl+Shift+Q`

Queue tasks for sequential automatic execution.

### Task Queue

| Display | Content |
|---------|---------|
| Task name | Task title |
| Status | pending / completed / failed / abandoned |
| Duration | Time taken |

| Button | Description |
|--------|-------------|
| **+ New Task** | Add a task |
| ✏ Edit | Modify a task |
| 🗑 Delete | Remove from queue |
| ▶ **Start Queue** | Begin sequential execution |
| ⏸ **Pause** | Pause after current task |
| ⏹ **Stop** | Stop completely; remaining tasks become skipped |

### Task Settings

| Field | Description |
|-------|-------------|
| Title | Task name |
| Message | Command or instruction to send |
| Target pane | Destination pane |
| **Clear before running** | Clear the pane before execution |
| Clear command | Command used to clear (e.g., `clear`) |

### Templates

Save and load frequently used task combinations as templates.

---

## 9. Single Task Runner

**Shortcut:** `Ctrl+Shift+J` (changed from `Ctrl+Shift+R` in v1.0.4)

A lightweight task runner that executes tasks sequentially on a single pane without requiring an orchestrator. Tasks can be queued, completed, and reported via MCP tools.

### Task List

| Display | Content |
|---------|---------|
| Task name | Task title |
| Status | pending / sending / active / done / failed / cancelled |
| Error message | Reason for failure |

| Button | Description |
|--------|-------------|
| **+ New Task** | Add a task to the queue |
| ▶ **Start Queue** | Begin sequential execution |
| ⏹ **Stop Queue** | Stop the queue |

### Task Form

| Field | Description |
|-------|-------------|
| Title | Task name |
| Message | Instruction to send to the pane |
| **Clear before running** | Clear the pane before execution |

### MCP Tools

Single Task Runner provides 6 MCP tools:

| Tool | Description |
|------|-------------|
| `enqueue_task` | Add a task to the queue |
| `complete_task` | Mark the current task as done |
| `fail_task` | Mark the current task as failed |
| `list_queue` | List all tasks in the queue |
| `cancel_task` | Cancel a task |
| `help` | Show tool list and help |

---

## 10. Orchestrator Teams

**Shortcut:** `Ctrl+Shift+T`

Manage AI agent team composition and launch.
See [Agent Teams](agent-teams_en.md) for details.

---

## 11. Error Log

**Shortcut:** `Ctrl+Shift+L`

Displays application Warn/Error level logs in real time.

| Display | Content |
|---------|---------|
| Timestamp | When the error occurred |
| Level | error / warn |
| Message | Error description |
| Source | Component name where the error originated |

### Header Buttons

| Button | Description |
|--------|-------------|
| Copy | Copy all entries to clipboard |
| Mark read | Clear unread badges |
| ↻ | Reload |
| ✕ | Close |

### Badge Notification

When unread errors exist, the Error Log icon in the Activity Strip shows a badge (●).

Logs are persisted in JSONL format, so they can be reviewed even after the session ends.

---

## 12. Usage Dashboard

**Shortcut:** `Ctrl+Shift+U`

Aggregates and visualizes Claude Code CLI and Codex CLI usage statistics scoped to the current session's working directory (added in v1.0.4).

### Tabs

| Tab | Source |
|-----|--------|
| **Claude** | `~/.claude/projects/**/*.jsonl` + `subagents/*.jsonl` |
| **Codex** | `.codex/sessions/**/*.jsonl` + `state_5.sqlite` |
| **Both** | Both sources side by side |

### Display

| Element | Description |
|---------|-------------|
| Overview cards | Total sessions, messages, tool calls |
| Top Agents | Most-invoked Agents |
| Top Skills | Most-used Skills |
| Top Slash Commands | Most-executed slash commands |
| Daily Activity | Past 30-day activity (recharts BarChart, added in v1.0.5) |
| Source Health Banner | Warns about read errors or missing data sources |

### Cache and Refresh

- Aggregated results are cached in `.myT-x/usage-dashboard.json` for fast re-access
- mtime-based diff detection avoids re-parsing unchanged files
- Manual refresh button forces re-aggregation
- SQLite access is read-only

---

## 13. Prompt Presets

**Shortcut:** `Ctrl+Shift+P`

Register and manage frequently used prompt bodies as "presets" (added in v1.0.6).
Append a preset body to the chat input text area via the **Prompt Presets** dropdown in the chat input bar.

### Scopes

| Scope | Storage |
|-------|---------|
| **Global** | `%LOCALAPPDATA%\myT-x\prompt-presets.json` |
| **Project** | `{session working directory}\.myT-x\prompt-presets.json` |

Project scope is only available when a session is selected.

### Operations

| Operation | Description |
|-----------|-------------|
| **+ New** | Add a new preset (name + body) |
| ✏ Edit | Update preset content |
| 🗑 Delete | Delete preset (confirmation required) |
| ↕ Reorder | Drag to change the dropdown display order |

### Constraints

| Item | Value |
|------|-------|
| Max presets per scope | 200 |
| ID format | UUID v4 |
| Write strategy | Atomic write (temp file → rename) |

---

## Next Steps

- [Settings](settings_en.md) — Including viewer shortcut customization
- [Agent Teams](agent-teams_en.md) — AI team development details
- [Shortcuts](shortcuts_en.md) — Complete keyboard shortcut reference
