# Troubleshooting

---

## Japanese Input (IME) Is Broken

### Symptoms
- Cannot convert Japanese characters; hiragana is sent as-is
- Confirmed characters are entered twice
- IME stays in English mode when returning focus to a pane

### Solution
1. **v1.0.9 and later**: Returning focus to the terminal pane automatically re-establishes the IME surface
2. If auto-recovery doesn't help, click the **A↻** (IME Reset) button in the menu bar
3. If that still doesn't help, restart the application

> myT-x uses WebView2 process isolation, so IME Reset usually resolves the issue.

---

## Settings Are Broken

### Symptoms
- Settings screen won't open or settings aren't applied

### Solution
- The app starts with **default settings** even if the config file has parse errors
- Fix the values from the settings screen
- Config file location: `%LOCALAPPDATA%\myT-x\config.yaml`

---

## Quake Mode Hotkey Doesn't Work

### Cause
Another application may be using the same key combination.

### Solution
1. Open the "General" tab in settings
2. Change the **Global Hotkey** to a different combination
3. Save and restart the app

---

## Pane Splitting Doesn't Work

### Cause
The prefix key may not have been pressed or recognized.

### Solution
1. Press `Ctrl+b` (default)
2. Confirm that **"PREFIX"** appears in the status bar
3. Then press the action key (`%`, `"`, etc.)

> If "PREFIX" doesn't appear, check the key assignment in the settings screen.

---

## Agent Teams Not Available

### Symptoms
- "(shim not installed)" is displayed
- Cannot create Agent Team sessions

### Solution
The tmux-shim may not be installed.
The application handles auto-installation, but PATH configuration may be needed.

---

## Error When Closing a Worktree Session

### Symptoms
- An error appears in the session close dialog

### Solution
The session close dialog offers these options:

| Button | Description |
|--------|-------------|
| **Close without saving** | Discard changes and close |
| **Commit then Close** | Commit changes before closing |
| **Push then Close** | Push committed changes before closing |
| **Commit & Push then Close** | Commit and push before closing |

You can also choose whether to **delete the Worktree** via a checkbox.

---

## Checking Error Logs

When problems occur, check the **Error Log** viewer (`Ctrl+Shift+L`) for details.

- A badge on the ⚠ icon in the Activity Strip indicates unread errors
- Logs are persisted in JSONL format, so they can be reviewed after the session ends

---

## System Requirements

| Item | Requirement |
|------|-------------|
| OS | Windows 10 / 11 |
| Runtime | WebView2 Runtime (typically bundled with Windows) |
