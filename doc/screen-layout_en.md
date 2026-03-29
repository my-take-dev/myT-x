# Screen Layout

## Overall Layout

The myT-x screen consists of 5 areas:

```
+------------------------------------------------------------+---+
|  (1) Menu Bar                                                |   |
+------------+-----------------------------------------------+ (5)|
|            |                                               |   |
|  (2) Left  |  (3) Main Area                                | A |
|  Sidebar   |                                               | c |
|            |  ┌──────────┬──────────┐                      | t |
|            |  │  Pane 1  │  Pane 2  │                      | i |
| ---------- |  │          │          │                      | v |
|            |  └──────────┴──────────┘                      | i |
| S Work A   |                                               | t |
| A Agent    |                                               | y |
|            |                                               |   |
| [+ New]    |                                               | S |
+------------+-----------------------------------------------+ t |
|  (4) Status Bar                                              | r |
+------------------------------------------------------------+---+
```

---

## 1. Menu Bar (Top)

| Button | Description |
|--------|-------------|
| **Settings** | Opens the settings screen for shell, key bindings, model configuration, etc. |
| **A↻** (IME Reset) | Restores Japanese input conversion when broken. Grayed out when not needed |
| **Japanese / English** | Switches the display language |

---

## 2. Left Sidebar

Displays the session list.

### Header
- **myT-x** logo and subtitle
- **+ New Session** button

### Session List

Each session shows:

| Display | Meaning |
|---------|---------|
| **S** / **A** badge | Session type (S = Standard, A = Agent Team) |
| Session name | Click to switch, double-click to rename |
| State badge | "Selected", "Running", or "Stopped" |
| Repository name | Shown when using Worktree |
| Branch name | Shown as `main ↦ feature/xxx` |

### Right-Click Menu

| Menu Item | Description |
|-----------|-------------|
| Promote to Branch | Convert detached HEAD to a named branch |
| Open in Explorer | Open working directory in Windows Explorer |
| Close session | Close the session (with commit/push confirmation for Worktree) |

---

## 3. Main Area (Center)

The main workspace where terminal panes are displayed.

### When No Session Is Selected
- "Create a session to get started" message
- **▶ Quick Start** button

### When a Session Is Selected

Terminal panes are arranged in the area. You can split them into multiple panes.

Each pane has a toolbar at the top:

| Button | Description |
|--------|-------------|
| **Pane ID** (%0, %1...) | Pane identifier used in Agent Teams to reference other panes |
| **Pane name** | Click to edit the name |
| **↻ Auto Enter** | Automatic input mode — sends a message when the pane becomes idle |
| **┃ Split Left/Right** | Split the pane horizontally |
| **━ Split Top/Bottom** | Split the pane vertically |
| **👤+ Add Member** | Add a member to the Agent Team |
| **✕ Close Pane** | Close the pane (red button) |

### Layout Presets

When you have multiple panes, use layout buttons for instant arrangement:

| Preset | Layout |
|--------|--------|
| Even Horizontal | All panes side by side |
| Even Vertical | All panes stacked |
| Main Left | Large pane on left, others stacked on right |
| Main Top | Large pane on top, others side by side below |
| Tiled | Grid arrangement |

### Canvas Mode

Toggle between standard terminal view and canvas mode.

In canvas mode, panes are shown as nodes with task flows visualized as connecting edges.
This makes Agent Team orchestration relationships easy to understand at a glance.

---

## 4. Status Bar (Bottom)

| Display | Meaning |
|---------|---------|
| Session:pane | Currently active session and pane |
| Pane title | Name of the selected pane |
| Time | Current time |
| **SYNC** | Shown when sync input mode is active |
| **PREFIX** | Shown when the prefix key has been pressed |

---

## 5. Activity Strip (Far Right)

A vertical bar of icons for opening viewers.

| Icon | Viewer | Shortcut |
|------|--------|----------|
| 📁 | File Tree | Ctrl+Shift+E |
| 🌿 | Git Graph | Ctrl+Shift+G |
| ± | Diff | Ctrl+Shift+D |
| 📜 | Input History | Ctrl+Shift+H |
| 🔌 | MCP Manager | Ctrl+Shift+M |
| ⏱ | Pane Scheduler | Ctrl+Shift+K |
| 📋 | Task Scheduler | Ctrl+Shift+Q |
| 👥 | Orchestrator Teams | Ctrl+Shift+T |
| ⚠ | Error Log | Ctrl+Shift+L |

- Click an icon to open its viewer; click again or press `Escape` to close
- Error Log shows a badge (●) when unread errors exist
- Use the toggle at the bottom to switch between **Docked** (side by side) and **Overlay** (floating) modes

---

## Chat Input Bar

Located at the bottom of the main area for sending messages to panes.

### Collapsed
- Shows pane number and name
- Click to expand

### Expanded (Floating Window)

| Element | Description |
|---------|-------------|
| Pane selector buttons | Choose which pane to send to |
| Text area | Enter your message (multiline supported) |
| **½** button | Toggle half-size overlay |
| Arrow buttons (↗↖↙↘) | Reposition the input panel |
| **Auto close** checkbox | Close the panel automatically after sending |
| **Send** button | Send the message (also Ctrl+Enter) |

---

## Next Steps

- [Terminal Operations](terminal-operations_en.md) — Detailed pane splitting and input guide
- [Viewer System](viewer-system_en.md) — All 9 viewers explained
- [Settings](settings_en.md) — Customization options
