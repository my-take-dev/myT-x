# Terminal Operations

## Splitting Panes

### From the Toolbar

Use the buttons in each pane's toolbar:

| Button | Action | Result |
|--------|--------|--------|
| ┃ | Split Left/Right | A new pane appears to the right |
| ━ | Split Top/Bottom | A new pane appears below |

### With the Prefix Key

1. Press `Ctrl+b` (prefix key) — "PREFIX" appears in the status bar
2. Then press an action key:

| Key | Action |
|-----|--------|
| `%` | Split left/right |
| `"` | Split top/bottom |
| `z` | Zoom (maximize / restore) |
| `x` | Close pane |
| `d` | Detach (disconnect while keeping the session alive) |
| `s` | Toggle sync input mode |

> The prefix key and action keys can be customized in the settings screen.

---

## Layout Presets

When you have multiple panes, use layout buttons for instant arrangement:

| Preset | Layout |
|--------|--------|
| **Even Horizontal** | `[A][B][C]` — All panes side by side |
| **Even Vertical** | `[A]` / `[B]` / `[C]` — All panes stacked |
| **Main Left** | `[A  ][B]` / `[A  ][C]` — Large pane on the left |
| **Main Top** | `[  A  ]` / `[B][C]` — Large pane on top |
| **Tiled** | `[A][B]` / `[C][D]` — Grid layout |

---

## Copy & Paste

| Action | How |
|--------|-----|
| Copy | Select text — it is copied automatically. `Ctrl+C` (with text selected) also works |
| Paste | Right-click, or `Ctrl+V` |
| Interrupt | `Ctrl+C` (with no text selected) |

---

## Change Font Size

Use `Ctrl` + mouse wheel to zoom in and out.

---

## Text Search

Press `Ctrl+F` to search within the pane output.
Useful for finding error messages in long log output.

| Feature | Description |
|---------|-------------|
| Incremental search | Matches are found in real-time as you type |
| Result counter | Shows current position and total matches (e.g. `3 / 17`) |
| Highlight decorations | Matching locations are visually highlighted in the terminal |
| IME support | Works with Japanese input composition |

---

## File Drop

Drag and drop a file onto a pane to paste its file path.

---

## Quick Search (Command Palette)

Press `Ctrl+P` or click the search button in the center of the Menu Bar.

| Feature | Description |
|---------|-------------|
| Session switching | Fuzzy-search by session name, repository name, or branch name |
| Viewer launch | Type a viewer name to open it directly |
| Command execution | Run common commands (e.g. open settings) |
| Template selection | Select task scheduler templates |

- Use arrow keys to navigate, `Enter` to execute
- `Escape` to close
- Menu Bar button opens as a dropdown; `Ctrl+P` opens as a centered palette

---

## Quake Mode (Quick Summon)

Press `Ctrl+Shift+F12` (customizable) to show or hide the window.

Call up the terminal only when you need it while working in a browser or editor.

> Toggle on/off and change the hotkey in Settings > General.

---

## Sync Input Mode

When enabled, input is sent to **all panes** in the current window simultaneously.

### How to Enable
- Press prefix key → `s`
- Or use the toggle button at the top of the session view

### Use Cases
- Run the same command across multiple panes
- Send the same instruction to all Agent Team members

**SYNC** appears in the status bar when active.

---

## Chat Input Bar

A dockable message-sending panel attached to the edge of the main area. Docks to top, bottom, left, or right, with a drag handle for resizing.

### Basic Usage

1. Click the input bar at the edge to expand
2. Select the target pane with the selector buttons
3. Type your message
4. Click **Send** or press `Ctrl+Enter`

### Options

| Option | Description |
|--------|-------------|
| **½** button | Toggle half-size panel |
| Anchor buttons (↑↓←→) | Change docking direction (top / bottom / left / right) |
| **Auto close** | When checked, the panel closes automatically after sending |

---

## Auto Enter

Enable via the **↻ Auto Enter** button in the pane toolbar.

When enabled, a configured message is automatically sent when the pane's process becomes idle.
Useful for repetitive CI/CD tasks or automated AI agent instructions.

---

## Editing Pane Names

Click the pane name in the toolbar to enter edit mode.

- `Enter` to confirm
- `Escape` to cancel

Helpful for identifying each pane's role during Agent Team orchestration.

---

## Next Steps

- [Viewer System](viewer-system_en.md) — File tree, Git graph, and other dev tools
- [Shortcuts](shortcuts_en.md) — Complete keyboard shortcut reference
