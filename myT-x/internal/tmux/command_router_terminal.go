package tmux

import (
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"myT-x/internal/terminal"
)

// PaneOutputEvent carries one terminal output chunk for frontend delivery.
type PaneOutputEvent struct {
	PaneID string
	Data   []byte
}

const maxCustomEnvValueBytes = 8192

// blockedEnvironmentKeys lists environment variable names that must not be
// overridden by client-supplied values. Overriding these could enable path
// hijacking or system directory redirection.
var blockedEnvironmentKeys = map[string]struct{}{
	"PATH":         {},
	"PATHEXT":      {},
	"COMSPEC":      {},
	"SYSTEMROOT":   {},
	"WINDIR":       {},
	"SYSTEMDRIVE":  {},
	"APPDATA":      {},
	"LOCALAPPDATA": {},
	"PSMODULEPATH": {},
	"TEMP":         {},
	"TMP":          {},
	"USERPROFILE":  {},
}

// isBlockedEnvironmentKey returns true if the given key (case-insensitive)
// must not be overridden by client-supplied values.
func isBlockedEnvironmentKey(key string) bool {
	_, blocked := blockedEnvironmentKeys[strings.ToUpper(key)]
	return blocked
}

func (r *CommandRouter) attachPaneTerminal(pane *TmuxPane, workDir string, env map[string]string, source *TmuxPane) error {
	if r.attachTerminalFn == nil {
		return fmt.Errorf("attach terminal function is not configured")
	}
	return r.attachTerminalFn(pane, workDir, env, source)
}

func (r *CommandRouter) attachTerminal(pane *TmuxPane, workDir string, env map[string]string, source *TmuxPane) error {
	shell := r.opts.DefaultShell
	if shell == "" {
		shell = "powershell.exe"
	}
	cols := pane.Width
	rows := pane.Height
	if cols <= 0 {
		cols = DefaultTerminalCols
	}
	if rows <= 0 {
		rows = DefaultTerminalRows
	}

	merged := mergeEnvironment(env)
	cfg := terminal.Config{
		Shell:   shell,
		Dir:     workDir,
		Env:     merged,
		Columns: cols,
		Rows:    rows,
	}
	t, err := terminal.Start(cfg)
	if err != nil {
		return err
	}
	sourceTitle := ""
	if source != nil {
		sourceTitle = source.Title
	}
	if bindErr := r.sessions.SetPaneRuntime(pane.ID, t, env, sourceTitle); bindErr != nil {
		if closeErr := t.Close(); closeErr != nil {
			slog.Warn("[terminal] attachTerminal: failed to close terminal after bind failure",
				"paneId", pane.IDString(),
				"bindErr", bindErr,
				"closeErr", closeErr,
			)
		}
		return bindErr
	}

	paneID := pane.IDString()
	slog.Info("[terminal] attachTerminal: starting ReadLoop", "paneId", paneID, "shell", shell)
	go func() {
		restartDelay := initialRouterPanicRestartBackoff
		for {
			panicked := false
			func() {
				defer func() {
					if recoverRouterPanic("pane-read-loop", recover()) {
						panicked = true
					}
				}()
				t.ReadLoop(func(chunk []byte) {
					defer func() {
						if recoverRouterPanic("pane-read-loop-callback", recover()) {
							slog.Warn("[DEBUG-PANIC] pane output callback panic recovered; continuing read loop",
								"paneId", paneID,
							)
						}
					}()
					slog.Debug("[terminal] ReadLoop output", "paneId", paneID, "chunkLen", len(chunk))
					r.emitter.Emit("tmux:pane-output", PaneOutputEvent{
						PaneID: paneID,
						Data:   chunk,
					})
				})
			}()
			if !panicked {
				return
			}
			if t.IsClosed() {
				slog.Debug("[DEBUG-PANIC] stopping pane read loop restart because terminal is closed",
					"paneId", paneID,
				)
				return
			}
			slog.Warn("[DEBUG-PANIC] restarting pane read loop after panic",
				"paneId", paneID,
				"restartDelay", restartDelay,
			)
			time.Sleep(restartDelay)
			restartDelay = nextRouterPanicRestartBackoff(restartDelay)
		}
	}()
	return nil
}

func addTmuxEnvironment(env map[string]string, pipeName string, hostPID int, sessionIndex int, paneID int, shimAvailable bool) {
	tmuxValue := fmt.Sprintf(`%s,%d,%d`, pipeName, hostPID, sessionIndex)
	paneValue := fmt.Sprintf("%%%d", paneID)

	// myT-x 内部用: 常に設定
	env["GO_TMUX"] = tmuxValue
	env["GO_TMUX_PANE"] = paneValue

	// 標準 tmux 互換変数: 常に設定。
	// 本物の tmux (environ.c:278, spawn.c:316) と同様に無条件で設定する。
	// シムは ensureShimReady で自動インストール・PATH登録されるため
	// 通常は常に利用可能。
	env["TMUX"] = tmuxValue
	env["TMUX_PANE"] = paneValue

	if !shimAvailable {
		slog.Warn("[shim] TMUX env set but shim not on PATH")
	}

	if username := os.Getenv("USERNAME"); username != "" {
		env["GO_TMUX_USER"] = username
	}
}

// mergePaneEnvDefaults merges paneEnv entries into env as lowest-priority
// defaults. Existing keys in env are never overwritten.
//
// Design: Key matching is case-sensitive. Different-case keys (e.g. "PATH" vs "path")
// coexist independently. Case-insensitive blocking of system keys (PATH, COMSPEC, etc.)
// is delegated to the downstream sanitizeCustomEnvironmentEntry, which uses
// strings.ToUpper for blocklist lookup. All callers MUST pass the resulting env
// through mergeEnvironment before process creation.
func mergePaneEnvDefaults(env map[string]string, paneEnv map[string]string) {
	if env == nil {
		slog.Error("[BUG] mergePaneEnvDefaults called with nil env map")
		return
	}
	for k, v := range paneEnv {
		if _, exists := env[k]; !exists {
			env[k] = v
		}
	}
}

func mergeEnvironment(custom map[string]string) []string {
	base := os.Environ()
	if len(custom) == 0 {
		return base
	}
	out := make(map[string]string, len(base)+len(custom))
	for _, item := range base {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		out[key] = value
	}
	for key, value := range custom {
		safeKey, safeValue, ok := sanitizeCustomEnvironmentEntry(key, value)
		if !ok {
			continue
		}
		out[safeKey] = safeValue
	}
	merged := make([]string, 0, len(out))
	for key, value := range out {
		merged = append(merged, fmt.Sprintf("%s=%s", key, value))
	}
	sort.Strings(merged)
	return merged
}

func sanitizeCustomEnvironmentEntry(key string, value string) (string, string, bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		slog.Debug("[sanitize] rejected env key", "key", key, "reason", "empty")
		return "", "", false
	}
	if strings.Contains(key, "=") {
		slog.Debug("[sanitize] rejected env key", "key", key, "reason", "contains '='")
		return "", "", false
	}
	if strings.ContainsRune(key, '\x00') {
		slog.Debug("[sanitize] rejected env key", "key", key, "reason", "contains null byte")
		return "", "", false
	}
	if _, blocked := blockedEnvironmentKeys[strings.ToUpper(key)]; blocked {
		slog.Warn("[sanitize] rejected blocked env key", "key", key)
		return "", "", false
	}

	origLen := len(value)
	value = strings.ReplaceAll(value, "\x00", "")
	if len(value) != origLen {
		slog.Warn("[sanitize] stripped null bytes from env value", "key", key)
	}
	if len(value) > maxCustomEnvValueBytes {
		slog.Warn("[sanitize] truncated env value", "key", key, "original", len(value), "limit", maxCustomEnvValueBytes)
		value = value[:maxCustomEnvValueBytes]
	}
	return key, value, true
}
