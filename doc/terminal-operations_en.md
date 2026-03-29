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

---

## File Drop

Drag and drop a file onto a pane to paste its file path.

---

## Quick Search

Press `Ctrl+P` to open the quick search palette.

- Fuzzy-search by session name, repository name, or branch name
- Use arrow keys to navigate, `Enter` to switch
- `Escape` to close

Quickly jump to any session when you have many open.

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

A floating input panel for sending messages to panes.

### Basic Usage

1. Click the input bar at the bottom to expand
2. Select the target pane with the selector buttons
3. Type your message
4. Click **Send** or press `Ctrl+Enter`

### Options

| Option | Description |
|--------|-------------|
| **½** button | Toggle half-size input area |
| Arrow buttons (↗↖↙↘) | Reposition the panel (top-right / top-left / bottom-left / bottom-right) |
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
