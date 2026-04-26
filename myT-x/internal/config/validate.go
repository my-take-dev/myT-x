package config

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"myT-x/internal/mcp"
)

const (
	// maxValidPort is the highest TCP/UDP port number (2^16 - 1).
	// Port 0 is valid and means "OS auto-assign".
	maxValidPort = 65535

	// maxCustomEnvValueBytes is the downstream limit enforced by CommandRouter.
	// Config layer warns early for values exceeding this threshold.
	// Why 8192: matches tmux.maxCustomEnvValueBytes for early user feedback.
	maxCustomEnvValueBytes = 8192
)

// TaskScheduler validation constants shared between config sanitizer and API layer.
const (
	MinPreExecResetDelay      = 0
	MaxPreExecResetDelay      = 60
	DefaultPreExecIdleTimeout = 30
	MinPreExecIdleTimeout     = 10
	MaxPreExecIdleTimeout     = 600
	MaxMessageTemplates       = 50
	MaxTemplateNameLen        = 100
	MaxTemplateMessageLen     = 5000
	MaxAutoStartCommands      = 50
	MaxAutoStartNameLen       = 100
	MaxAutoStartCommandLen    = 200
	MaxAutoStartArgsLen       = 1000
	defaultCustomMCPKind      = string(mcp.DefinitionKindCustom)
)

// allowedShells is the set of permitted shell executables (matched by base
// name, case-insensitive). Additions require security review to prevent
// arbitrary command execution.
var allowedShells = map[string]struct{}{
	"powershell.exe": {},
	"pwsh.exe":       {},
	"cmd.exe":        {},
	"bash.exe":       {},
	"wsl.exe":        {},
}

var shellNameAliases = map[string]string{
	"powershell": "powershell.exe",
	"pwsh":       "pwsh.exe",
	"cmd":        "cmd.exe",
	"bash":       "bash.exe",
	"wsl":        "wsl.exe",
}

var functionKeyPattern = regexp.MustCompile(`^f(?:[1-9]|1[0-9]|2[0-4])$`)

var viewerShortcutAliases = map[string]string{
	"file-tree": "file-view",
}

var reservedViewerShortcuts = map[string]string{
	"ctrl+shift+v": "file-content-preview-toggle",
}

var viewerShortcutDefinitions = []struct {
	viewID          string
	defaultShortcut string
}{
	{viewID: "file-view", defaultShortcut: "Ctrl+Shift+E"},
	{viewID: "git-graph", defaultShortcut: "Ctrl+Shift+G"},
	{viewID: "error-log", defaultShortcut: "Ctrl+Shift+L"},
	{viewID: "diff", defaultShortcut: "Ctrl+Shift+D"},
	{viewID: "input-history", defaultShortcut: "Ctrl+Shift+H"},
	{viewID: "mcp-manager", defaultShortcut: "Ctrl+Shift+M"},
	{viewID: "pane-scheduler", defaultShortcut: "Ctrl+Shift+K"},
	{viewID: "prompt-presets", defaultShortcut: "Ctrl+Shift+P"},
	{viewID: "single-task-runner", defaultShortcut: "Ctrl+Shift+J"},
	{viewID: "task-scheduler", defaultShortcut: "Ctrl+Shift+Q"},
	{viewID: "editor", defaultShortcut: "Ctrl+Shift+O"},
	{viewID: "orchestrator-teams", defaultShortcut: "Ctrl+Shift+T"},
	{viewID: "usage-dashboard", defaultShortcut: "Ctrl+Shift+U"},
}

// AllowedShellList returns the permitted shell executable names for UI display.
// The result is sorted alphabetically for consistent ordering.
func AllowedShellList() []string {
	shells := make([]string, 0, len(allowedShells))
	for s := range allowedShells {
		shells = append(shells, s)
	}
	sort.Strings(shells)
	return shells
}

// warnOnlyBlockedKeys lists system environment keys that should not be
// overridden. This is a config-layer early-warning subset; the authoritative
// blocklist lives in tmux.blockedEnvironmentKeys and is enforced at process
// creation time by mergeEnvironment -> sanitizeCustomEnvironmentEntry.
var warnOnlyBlockedKeys = map[string]struct{}{
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

// BlockedKeyNames returns the set of environment variable names that the
// config layer warns about. This is exported for guard tests that verify
// consistency with tmux.blockedEnvironmentKeys.
// INVARIANT: Must stay in sync with tmux.blockedEnvironmentKeys and
// frontend settingsValidation.ts BLOCKED_ENV_KEYS.
func BlockedKeyNames() map[string]struct{} {
	cp := make(map[string]struct{}, len(warnOnlyBlockedKeys))
	maps.Copy(cp, warnOnlyBlockedKeys)
	return cp
}

// applyDefaultsAndValidate fills missing defaults and validates cfg in-place.
// MUTATES: cfg is directly modified.
// Used by both Load and Save to ensure consistent normalization.
func applyDefaultsAndValidate(cfg *Config) error {
	defaults := DefaultConfig()
	if isZeroConfig(*cfg) {
		*cfg = defaults
		sanitizePaneEnv(cfg)
		sanitizeClaudeEnv(cfg)
		sanitizeMCPServers(cfg)
		return normalizeAndValidateAgentModel(cfg.AgentModel)
	}

	if cfg.Shell == "" {
		cfg.Shell = defaults.Shell
	}
	if err := validateShell(cfg.Shell); err != nil {
		return err
	}
	if cfg.Prefix == "" {
		cfg.Prefix = defaults.Prefix
	}
	if strings.TrimSpace(cfg.GlobalHotkey) == "" {
		cfg.GlobalHotkey = defaults.GlobalHotkey
	}
	if cfg.Keys == nil {
		cfg.Keys = defaults.Keys
	}
	if cfg.AutoStart == nil {
		cfg.AutoStart = append([]AutoStartCommand(nil), defaults.AutoStart...)
	}
	if strings.TrimSpace(cfg.ViewerSidebarMode) == "" {
		cfg.ViewerSidebarMode = defaults.ViewerSidebarMode
	}
	if cfg.Worktree.SetupScripts == nil {
		cfg.Worktree.SetupScripts = append([]string(nil), defaults.Worktree.SetupScripts...)
	}
	if cfg.Worktree.SetupScriptTimeoutSeconds == 0 {
		cfg.Worktree.SetupScriptTimeoutSeconds = defaults.Worktree.SetupScriptTimeoutSeconds
	} else if cfg.Worktree.SetupScriptTimeoutSeconds < 0 {
		slog.Warn("[WARN-CONFIG] worktree.setup_script_timeout_seconds must be positive, resetting to default",
			"configured", cfg.Worktree.SetupScriptTimeoutSeconds,
			"default", defaults.Worktree.SetupScriptTimeoutSeconds)
		cfg.Worktree.SetupScriptTimeoutSeconds = defaults.Worktree.SetupScriptTimeoutSeconds
	}
	if cfg.Worktree.CopyFiles == nil {
		cfg.Worktree.CopyFiles = append([]string(nil), defaults.Worktree.CopyFiles...)
	}
	if cfg.Worktree.CopyDirs == nil {
		cfg.Worktree.CopyDirs = append([]string(nil), defaults.Worktree.CopyDirs...)
	}
	if err := normalizeAndValidateAgentModel(cfg.AgentModel); err != nil {
		return err
	}
	validateWebSocketPort(cfg)
	validateViewerSidebarMode(cfg)
	validateChatOverlayPercentage(cfg)
	sanitizeViewerHotkeys(cfg)
	sanitizeAutoStart(cfg)
	sanitizePaneEnv(cfg)
	sanitizeClaudeEnv(cfg)
	sanitizeMCPServers(cfg)
	sanitizeTaskScheduler(cfg)
	validateDefaultSessionDir(cfg)
	return nil
}

// NormalizeAutoStartCommand trims and validates one AutoStart entry.
// It returns false when the entry is not runnable and should be dropped.
func NormalizeAutoStartCommand(entry AutoStartCommand) (AutoStartCommand, bool) {
	entry.Name = sanitizeAutoStartField(entry.Name)
	entry.Command = sanitizeAutoStartField(entry.Command)
	entry.Args = sanitizeAutoStartField(entry.Args)

	if entry.Command == "" {
		return AutoStartCommand{}, false
	}
	if utf8.RuneCountInString(entry.Name) > MaxAutoStartNameLen {
		entry.Name = truncateRunes(entry.Name, MaxAutoStartNameLen)
	}
	if utf8.RuneCountInString(entry.Command) > MaxAutoStartCommandLen {
		entry.Command = truncateRunes(entry.Command, MaxAutoStartCommandLen)
	}
	if utf8.RuneCountInString(entry.Args) > MaxAutoStartArgsLen {
		entry.Args = truncateRunes(entry.Args, MaxAutoStartArgsLen)
	}
	return entry, true
}

func sanitizeAutoStart(cfg *Config) {
	if len(cfg.AutoStart) == 0 {
		if cfg.AutoStart == nil {
			cfg.AutoStart = []AutoStartCommand{}
		}
		return
	}

	seen := make(map[string]struct{}, len(cfg.AutoStart))
	filtered := make([]AutoStartCommand, 0, min(len(cfg.AutoStart), MaxAutoStartCommands))
	for i, entry := range cfg.AutoStart {
		normalized, ok := NormalizeAutoStartCommand(entry)
		if !ok {
			slog.Warn("[WARN-CONFIG] auto_start entry has empty command, skipping", "index", i)
			continue
		}

		key := strings.ToLower(normalized.Command) + "\x00" + normalized.Args
		if _, exists := seen[key]; exists {
			slog.Warn("[WARN-CONFIG] auto_start entry duplicates another command and arguments, skipping",
				"command", normalized.Command, "index", i)
			continue
		}
		seen[key] = struct{}{}
		filtered = append(filtered, normalized)
		if len(filtered) == MaxAutoStartCommands {
			if i < len(cfg.AutoStart)-1 {
				slog.Warn("[WARN-CONFIG] auto_start exceeds maximum, truncating",
					"count", len(cfg.AutoStart), "max", MaxAutoStartCommands)
			}
			break
		}
	}
	cfg.AutoStart = filtered
}

func sanitizeAutoStartField(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
	return strings.TrimSpace(value)
}

func truncateRunes(value string, maxRunes int) string {
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes])
}

// normalizeAndValidateAgentModel validates and normalizes agent model settings.
func normalizeAndValidateAgentModel(am *AgentModel) error {
	if am == nil {
		return nil
	}
	am.From = strings.TrimSpace(am.From)
	am.To = strings.TrimSpace(am.To)
	if (am.From == "" && am.To != "") || (am.From != "" && am.To == "") {
		return errors.New("agent_model: both 'from' and 'to' must be specified together")
	}
	for i := range am.Overrides {
		name := strings.TrimSpace(am.Overrides[i].Name)
		nameLen := utf8.RuneCountInString(name)
		if nameLen < minOverrideNameLen {
			return fmt.Errorf("agent_model.overrides[%d].name must be >= %d characters, got %q (%d chars)", i, minOverrideNameLen, name, nameLen)
		}
		model := strings.TrimSpace(am.Overrides[i].Model)
		if model == "" {
			return fmt.Errorf("agent_model.overrides[%d].model must not be empty", i)
		}
		am.Overrides[i].Name = name
		am.Overrides[i].Model = model
	}
	return nil
}

// validateWebSocketPort checks that WebSocketPort is within the valid TCP port
// range (0-65535). Port 0 means "let the OS auto-assign an available port".
// Invalid values are logged and reset to 0 (auto-assign) to keep the
// application startable even with a misconfigured config file.
// NOTE: non-fatal — invalid port falls back to default (0) instead of
// returning an error, consistent with the project policy that parse errors
// must not prevent startup.
func validateWebSocketPort(cfg *Config) {
	if cfg.WebSocketPort < 0 || cfg.WebSocketPort > maxValidPort {
		slog.Warn("[WARN-CONFIG] websocket_port out of valid range (0-65535), falling back to 0 (auto-assign)",
			"configured", cfg.WebSocketPort, "max", maxValidPort)
		cfg.WebSocketPort = 0
	}
}

// validateViewerSidebarMode normalizes viewer_sidebar_mode in place.
// Invalid values fall back to the default overlay behavior without failing startup.
func validateViewerSidebarMode(cfg *Config) {
	configuredMode := cfg.ViewerSidebarMode
	mode := strings.TrimSpace(configuredMode)
	switch mode {
	case "", "overlay", "docked":
		cfg.ViewerSidebarMode = mode
	default:
		slog.Warn("[WARN-CONFIG] viewer_sidebar_mode is invalid, falling back to overlay",
			"configured", configuredMode)
		cfg.ViewerSidebarMode = "overlay"
	}
}

// validateChatOverlayPercentage clamps ChatOverlayPercentage to the valid
// range defined by the exported chat overlay validation constants.
func validateChatOverlayPercentage(cfg *Config) {
	if cfg.ChatOverlayPercentage == 0 {
		cfg.ChatOverlayPercentage = DefaultChatOverlayPercentage
	}
	if cfg.ChatOverlayPercentage < MinChatOverlayPercentage {
		slog.Warn("[WARN-CONFIG] chat_overlay_percentage too low, clamping to minimum",
			"configured", cfg.ChatOverlayPercentage)
		cfg.ChatOverlayPercentage = MinChatOverlayPercentage
	}
	if cfg.ChatOverlayPercentage > MaxChatOverlayPercentage {
		slog.Warn("[WARN-CONFIG] chat_overlay_percentage too high, clamping to maximum",
			"configured", cfg.ChatOverlayPercentage)
		cfg.ChatOverlayPercentage = MaxChatOverlayPercentage
	}
}

// validateDefaultSessionDir normalizes DefaultSessionDir in place.
// Expands ~ prefix to the user's home directory, applies filepath.Clean,
// and clears non-absolute paths with a warning log (non-fatal).
func validateDefaultSessionDir(cfg *Config) {
	validateDefaultSessionDirWith(os.UserHomeDir, cfg)
}

// validateDefaultSessionDirWith is the parameterized implementation of validateDefaultSessionDir,
// allowing tests to inject test doubles for os.UserHomeDir.
func validateDefaultSessionDirWith(userHomeDirFn func() (string, error), cfg *Config) {
	dir := strings.TrimSpace(cfg.DefaultSessionDir)
	if dir == "" {
		cfg.DefaultSessionDir = ""
		return
	}
	// Expand ~ prefix to user home directory.
	if strings.HasPrefix(dir, "~") {
		home, err := userHomeDirFn()
		if err != nil {
			slog.Warn("[WARN-CONFIG] default_session_dir: failed to expand ~, ignoring",
				"path", dir, "error", err)
			cfg.DefaultSessionDir = ""
			return
		}
		dir = filepath.Join(home, dir[1:])
	}
	// Expand environment variables for parity with shell-style path inputs.
	dir = expandDefaultSessionDirEnv(dir)
	dir = filepath.Clean(dir)
	if !filepath.IsAbs(dir) {
		slog.Warn("[WARN-CONFIG] default_session_dir is not an absolute path, ignoring", "path", dir)
		cfg.DefaultSessionDir = ""
		return
	}
	cfg.DefaultSessionDir = dir
}

// validateShell ensures the configured shell is safe for process creation.
// It rejects null bytes, verifies the base name against allowedShells,
// confirms absolute paths exist on disk, and rejects relative paths that
// could resolve to unintended executables.
func validateShell(shell string) error {
	shell = strings.TrimSpace(shell)
	if shell == "" {
		return errors.New("shell is required")
	}
	if strings.ContainsRune(shell, '\x00') {
		return errors.New("shell contains invalid null byte")
	}

	baseName := CanonicalShellBaseName(shell)
	if _, ok := allowedShells[baseName]; !ok {
		return fmt.Errorf("shell %q is not in the allowlist", shell)
	}

	if filepath.IsAbs(shell) {
		info, err := os.Stat(shell)
		if err != nil {
			return fmt.Errorf("shell path does not exist: %w", err)
		}
		if info.IsDir() {
			return errors.New("shell path cannot be a directory")
		}
		return nil
	}

	// Reject relative paths such as ".\\tool\\shell.exe".
	if strings.Contains(shell, `\`) || strings.Contains(shell, "/") {
		return errors.New("shell must be executable name or absolute path")
	}
	return nil
}

// CanonicalShellBaseName normalizes a configured shell name to the allowlist
// base name used by config validation and runtime execution.
func CanonicalShellBaseName(shell string) string {
	baseName := strings.ToLower(strings.TrimSpace(filepath.Base(shell)))
	if canonical, ok := shellNameAliases[baseName]; ok {
		return canonical
	}
	return baseName
}

func canonicalizeViewerShortcutConfig(shortcuts map[string]string) map[string]string {
	if len(shortcuts) == 0 {
		return shortcuts
	}

	for legacyViewID, canonicalViewID := range viewerShortcutAliases {
		if direct, ok := shortcuts[canonicalViewID]; !ok || strings.TrimSpace(direct) == "" {
			if legacy, legacyExists := shortcuts[legacyViewID]; legacyExists && strings.TrimSpace(legacy) != "" {
				shortcuts[canonicalViewID] = legacy
			}
		}
		delete(shortcuts, legacyViewID)
	}
	return shortcuts
}

func normalizeShortcut(rawShortcut string) string {
	tokens := strings.Split(rawShortcut, "+")
	if len(tokens) == 0 {
		return ""
	}

	modifiers := make(map[string]struct{}, 4)
	key := ""
	for _, rawToken := range tokens {
		token := strings.ToLower(strings.TrimSpace(rawToken))
		if token == "" {
			continue
		}
		switch token {
		case "ctrl", "control":
			modifiers["ctrl"] = struct{}{}
		case "shift":
			modifiers["shift"] = struct{}{}
		case "alt", "option":
			modifiers["alt"] = struct{}{}
		case "meta", "cmd", "command":
			modifiers["meta"] = struct{}{}
		default:
			key = token
		}
	}
	if key == "" {
		return ""
	}

	orderedModifiers := make([]string, 0, len(modifiers)+1)
	for _, modifier := range []string{"ctrl", "shift", "alt", "meta"} {
		if _, ok := modifiers[modifier]; ok {
			orderedModifiers = append(orderedModifiers, modifier)
		}
	}
	orderedModifiers = append(orderedModifiers, key)
	return strings.Join(orderedModifiers, "+")
}

func isFunctionKeyToken(token string) bool {
	return functionKeyPattern.MatchString(token)
}

func hasShortcutModifier(shortcut string) bool {
	normalizedShortcut := normalizeShortcut(shortcut)
	if normalizedShortcut == "" {
		return false
	}
	tokens := strings.Split(normalizedShortcut, "+")
	if len(tokens) == 1 {
		return isFunctionKeyToken(tokens[0])
	}
	return len(tokens) >= 2
}

func formatShortcutDisplay(shortcut string) string {
	if shortcut == "" {
		return ""
	}

	labels := map[string]string{
		"ctrl":  "Ctrl",
		"shift": "Shift",
		"alt":   "Alt",
		"meta":  "Meta",
	}
	tokens := strings.Split(shortcut, "+")
	formatted := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if label, ok := labels[token]; ok {
			formatted = append(formatted, label)
			continue
		}
		if isFunctionKeyToken(token) {
			formatted = append(formatted, strings.ToUpper(token))
			continue
		}
		if len(token) == 1 {
			formatted = append(formatted, strings.ToUpper(token))
			continue
		}
		formatted = append(formatted, token)
	}
	return strings.Join(formatted, "+")
}

func sanitizeViewerHotkeys(cfg *Config) {
	normalizedGlobalHotkey := normalizeShortcut(cfg.GlobalHotkey)
	if cfg.QuakeMode && normalizedGlobalHotkey != "" {
		if reservedID, reserved := reservedViewerShortcuts[normalizedGlobalHotkey]; reserved {
			slog.Warn("[WARN-CONFIG] global_hotkey uses a reserved viewer shortcut, falling back to default",
				"configured", cfg.GlobalHotkey,
				"reservedID", reservedID,
				"default", DefaultConfig().GlobalHotkey)
			cfg.GlobalHotkey = DefaultConfig().GlobalHotkey
			normalizedGlobalHotkey = normalizeShortcut(cfg.GlobalHotkey)
		}
	}

	if len(cfg.ViewerShortcuts) == 0 {
		return
	}

	shortcuts := canonicalizeViewerShortcutConfig(cfg.ViewerShortcuts)
	cleaned := make(map[string]string, len(shortcuts))
	ownersByShortcut := make(map[string]string, len(viewerShortcutDefinitions))
	for _, definition := range viewerShortcutDefinitions {
		defaultShortcut := normalizeShortcut(definition.defaultShortcut)
		configuredShortcut, hasConfigured := shortcuts[definition.viewID]
		configuredShortcut = strings.TrimSpace(configuredShortcut)
		effectiveShortcut := defaultShortcut
		keepConfigured := false

		if hasConfigured && configuredShortcut != "" {
			normalizedShortcut := normalizeShortcut(configuredShortcut)
			switch {
			case normalizedShortcut == "":
				slog.Warn("[WARN-CONFIG] viewer_shortcuts entry uses an invalid shortcut, dropping",
					"viewID", definition.viewID, "configured", configuredShortcut)
			case !hasShortcutModifier(normalizedShortcut):
				slog.Warn("[WARN-CONFIG] viewer_shortcuts entry is missing a modifier key, dropping",
					"viewID", definition.viewID, "configured", configuredShortcut)
			case normalizedGlobalHotkey != "" && normalizedShortcut == normalizedGlobalHotkey:
				slog.Warn("[WARN-CONFIG] viewer_shortcuts entry conflicts with global_hotkey, dropping",
					"viewID", definition.viewID,
					"configured", configuredShortcut,
					"globalHotkey", cfg.GlobalHotkey)
			case reservedViewerShortcuts[normalizedShortcut] != "":
				slog.Warn("[WARN-CONFIG] viewer_shortcuts entry uses a reserved shortcut, dropping",
					"viewID", definition.viewID,
					"configured", configuredShortcut,
					"reservedID", reservedViewerShortcuts[normalizedShortcut])
			case ownersByShortcut[normalizedShortcut] != "":
				slog.Warn("[WARN-CONFIG] viewer_shortcuts entry duplicates another effective shortcut, dropping",
					"viewID", definition.viewID,
					"configured", configuredShortcut,
					"existingOwner", ownersByShortcut[normalizedShortcut])
			default:
				effectiveShortcut = normalizedShortcut
				keepConfigured = true
			}
		}

		if effectiveShortcut == "" {
			continue
		}
		if _, exists := ownersByShortcut[effectiveShortcut]; !exists {
			ownersByShortcut[effectiveShortcut] = definition.viewID
		}
		if keepConfigured {
			cleaned[definition.viewID] = formatShortcutDisplay(effectiveShortcut)
		}
	}

	if len(cleaned) == 0 {
		cfg.ViewerShortcuts = nil
		return
	}
	cfg.ViewerShortcuts = cleaned
}

// sanitizeTaskScheduler validates and normalizes task scheduler settings in place.
// Invalid values fall back to defaults without failing startup.
func sanitizeTaskScheduler(cfg *Config) {
	ts := cfg.TaskScheduler
	if ts == nil {
		return
	}

	if ts.PreExecResetDelay < MinPreExecResetDelay || ts.PreExecResetDelay > MaxPreExecResetDelay {
		slog.Warn("[WARN-CONFIG] task_scheduler.pre_exec_reset_delay_s out of range, resetting to 0",
			"configured", ts.PreExecResetDelay, "min", MinPreExecResetDelay, "max", MaxPreExecResetDelay)
		ts.PreExecResetDelay = 0
	}
	if ts.PreExecIdleTimeout == 0 {
		ts.PreExecIdleTimeout = DefaultPreExecIdleTimeout
	}
	if ts.PreExecIdleTimeout < MinPreExecIdleTimeout || ts.PreExecIdleTimeout > MaxPreExecIdleTimeout {
		slog.Warn("[WARN-CONFIG] task_scheduler.pre_exec_idle_timeout_s out of range, resetting to default 30",
			"configured", ts.PreExecIdleTimeout, "min", MinPreExecIdleTimeout, "max", MaxPreExecIdleTimeout)
		ts.PreExecIdleTimeout = DefaultPreExecIdleTimeout
	}

	if ts.PreExecTargetMode == "" {
		ts.PreExecTargetMode = TaskSchedulerPreExecTargetModeTaskPanes
	} else if !IsValidTaskSchedulerPreExecTargetMode(ts.PreExecTargetMode) {
		slog.Warn("[WARN-CONFIG] task_scheduler.pre_exec_target_mode is invalid, falling back to task_panes",
			"configured", ts.PreExecTargetMode)
		ts.PreExecTargetMode = TaskSchedulerPreExecTargetModeTaskPanes
	}

	if len(ts.MessageTemplates) > 0 {
		seen := make(map[string]struct{}, len(ts.MessageTemplates))
		filtered := make([]MessageTemplate, 0, len(ts.MessageTemplates))
		for i, tmpl := range ts.MessageTemplates {
			tmpl.Name = strings.TrimSpace(tmpl.Name)
			tmpl.Message = strings.TrimSpace(tmpl.Message)
			if tmpl.Name == "" {
				slog.Warn("[WARN-CONFIG] task_scheduler.message_templates entry has empty name, skipping",
					"index", i)
				continue
			}
			if tmpl.Message == "" {
				slog.Warn("[WARN-CONFIG] task_scheduler.message_templates entry has empty message, skipping",
					"name", tmpl.Name)
				continue
			}
			if utf8.RuneCountInString(tmpl.Name) > MaxTemplateNameLen {
				slog.Warn("[WARN-CONFIG] task_scheduler.message_templates entry name exceeds maximum length, skipping",
					"name", tmpl.Name, "max", MaxTemplateNameLen, "index", i)
				continue
			}
			if utf8.RuneCountInString(tmpl.Message) > MaxTemplateMessageLen {
				slog.Warn("[WARN-CONFIG] task_scheduler.message_templates entry message exceeds maximum length, skipping",
					"name", tmpl.Name, "max", MaxTemplateMessageLen, "index", i)
				continue
			}
			if _, exists := seen[tmpl.Name]; exists {
				slog.Warn("[WARN-CONFIG] task_scheduler.message_templates entry has duplicate name, skipping",
					"name", tmpl.Name, "index", i)
				continue
			}
			seen[tmpl.Name] = struct{}{}
			filtered = append(filtered, tmpl)
		}
		if len(filtered) > MaxMessageTemplates {
			slog.Warn("[WARN-CONFIG] task_scheduler.message_templates exceeds maximum after sanitization, truncating",
				"count", len(filtered), "max", MaxMessageTemplates)
			filtered = filtered[:MaxMessageTemplates]
		}
		ts.MessageTemplates = filtered
	}
}

// sanitizePaneEnv removes invalid entries from PaneEnv using sanitizeEnvMap.
// Blocked-key validation is deferred to CommandRouter's sanitizeCustomEnvironmentEntry.
func sanitizePaneEnv(cfg *Config) {
	cfg.PaneEnv = sanitizeEnvMap(cfg.PaneEnv, "pane_env")
}

// sanitizeClaudeEnv removes invalid entries from ClaudeEnv.Vars using sanitizeEnvMap.
// Operates on cfg.ClaudeEnv.Vars; keeps the struct for DefaultEnabled even when
// all vars are removed.
func sanitizeClaudeEnv(cfg *Config) {
	if cfg.ClaudeEnv == nil {
		return
	}
	cfg.ClaudeEnv.Vars = sanitizeEnvMap(cfg.ClaudeEnv.Vars, "claude_env")
}

// sanitizeMCPServers validates and normalizes mcp_servers entries in place.
// Invalid entries are skipped with warning logs to keep config loading non-fatal.
func sanitizeMCPServers(cfg *Config) {
	if len(cfg.MCPServers) == 0 {
		return
	}
	seen := make(map[string]struct{}, len(cfg.MCPServers))
	filtered := make([]MCPServerConfig, 0, len(cfg.MCPServers))
	for i := range cfg.MCPServers {
		server := cfg.MCPServers[i]
		server.ID = strings.TrimSpace(server.ID)
		server.Name = strings.TrimSpace(server.Name)
		server.Description = strings.TrimSpace(server.Description)
		server.Kind = strings.TrimSpace(server.Kind)
		server.Command = strings.TrimSpace(server.Command)
		server.UsageSample = strings.TrimSpace(server.UsageSample)

		if server.ID == "" {
			slog.Warn("[WARN-CONFIG] mcp_servers entry has empty id, skipping", "index", i)
			continue
		}
		if server.Name == "" {
			slog.Warn("[WARN-CONFIG] mcp_servers entry has empty name, skipping", "id", server.ID)
			continue
		}
		if server.Command == "" {
			slog.Warn("[WARN-CONFIG] mcp_servers entry has empty command, skipping", "id", server.ID)
			continue
		}
		if server.Kind == "" {
			server.Kind = defaultCustomMCPKind
		}
		if isReservedConfigMCPKind(server.Kind) {
			slog.Warn("[WARN-CONFIG] mcp_servers entry uses reserved built-in kind, skipping",
				"id", server.ID, "kind", server.Kind)
			continue
		}
		if _, exists := seen[server.ID]; exists {
			slog.Warn("[WARN-CONFIG] mcp_servers entry has duplicate id, skipping", "id", server.ID, "index", i)
			continue
		}
		seen[server.ID] = struct{}{}

		if len(server.Args) > 0 {
			trimmedArgs := make([]string, 0, len(server.Args))
			for argIdx, arg := range server.Args {
				arg = strings.TrimSpace(arg)
				if arg == "" {
					slog.Warn("[WARN-CONFIG] mcp_servers entry has empty args item, skipping", "id", server.ID, "argIndex", argIdx)
					continue
				}
				trimmedArgs = append(trimmedArgs, arg)
			}
			server.Args = trimmedArgs
		}
		server.Env = sanitizeEnvMap(server.Env, fmt.Sprintf("mcp_servers[%d].env", i))
		server.ConfigParams = sanitizeMCPServerConfigParams(server.ConfigParams, server.ID)

		filtered = append(filtered, server)
	}
	cfg.MCPServers = filtered
}

func isReservedConfigMCPKind(kind string) bool {
	switch mcp.DefinitionKind(strings.TrimSpace(kind)) {
	case mcp.DefinitionKindOrchestrator, mcp.DefinitionKindSingleTaskRunner:
		return true
	default:
		return false
	}
}

func sanitizeMCPServerConfigParams(params []MCPServerConfigParam, mcpID string) []MCPServerConfigParam {
	if len(params) == 0 {
		return nil
	}
	filtered := make([]MCPServerConfigParam, 0, len(params))
	for i := range params {
		param := params[i]
		param.Key = strings.TrimSpace(param.Key)
		param.Label = strings.TrimSpace(param.Label)
		param.DefaultValue = strings.TrimSpace(param.DefaultValue)
		param.Description = strings.TrimSpace(param.Description)
		if param.Key == "" {
			slog.Warn("[WARN-CONFIG] mcp_servers entry has config_params item with empty key, skipping", "id", mcpID, "index", i)
			continue
		}
		if param.Label == "" {
			slog.Warn("[WARN-CONFIG] mcp_servers entry has config_params item with empty label, skipping", "id", mcpID, "index", i)
			continue
		}
		filtered = append(filtered, param)
	}
	return filtered
}

// sanitizeEnvMap validates and cleans environment variable entries.
// It removes entries with empty keys, null bytes in keys, '=' in keys,
// and strips null bytes from values. Values are trimmed but allowed to be empty
// (note: GUI saves skip empty values, so empty values only appear when set via
// config.yaml directly).
// Duplicate key detection is case-insensitive (Windows env vars are case-insensitive),
// keeping the first occurrence's original case (sorted alphabetically for determinism).
// Returns nil when the input is empty or all entries are removed.
func sanitizeEnvMap(entries map[string]string, logPrefix string) map[string]string {
	if len(entries) == 0 {
		return nil
	}
	cleaned := make(map[string]string, len(entries))
	// seen tracks uppercase keys for case-insensitive duplicate detection.
	seen := make(map[string]string, len(entries)) // uppercase -> original key
	// Sort keys for deterministic duplicate resolution (Go map iteration is random).
	sortedKeys := make([]string, 0, len(entries))
	for k := range entries {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)
	for _, k := range sortedKeys {
		v := entries[k]
		k = strings.TrimSpace(k)
		if k == "" {
			slog.Debug("[DEBUG-CONFIG] " + logPrefix + ": dropped entry with empty key")
			continue
		}
		if strings.ContainsRune(k, '\x00') {
			slog.Warn("[WARN-CONFIG] "+logPrefix+": dropped entry with null byte in key", "key", k)
			continue
		}
		// Windows environment variable keys cannot contain '='.
		if strings.ContainsRune(k, '=') {
			slog.Warn("[WARN-CONFIG] "+logPrefix+": dropped entry with '=' in key", "key", k)
			continue
		}
		// Early warning for blocked system keys (actual enforcement is downstream).
		if _, blocked := warnOnlyBlockedKeys[strings.ToUpper(k)]; blocked {
			slog.Warn("[WARN-CONFIG] "+logPrefix+": blocked system key will be rejected at process creation", "key", k)
		}
		origLen := len(v)
		v = strings.ReplaceAll(v, "\x00", "")
		if len(v) != origLen {
			slog.Warn("[WARN-CONFIG] "+logPrefix+": stripped null bytes from value", "key", k)
		}
		v = strings.TrimSpace(v)
		// Case-insensitive duplicate detection (Windows env vars are case-insensitive).
		upperK := strings.ToUpper(k)
		if firstKey, exists := seen[upperK]; exists {
			slog.Warn("[WARN-CONFIG] "+logPrefix+": duplicate key (case-insensitive), keeping first", "key", k, "kept", firstKey)
			continue // first-wins
		}
		// Early warning for excessively long values (downstream enforces hard limit).
		if len(v) > maxCustomEnvValueBytes {
			slog.Warn("[WARN-CONFIG] "+logPrefix+": value exceeds recommended limit", "key", k, "bytes", len(v), "limit", maxCustomEnvValueBytes)
		}
		seen[upperK] = k
		cleaned[k] = v
	}
	// Normalize to nil when all entries were removed, consistent with Clone() behavior.
	if len(cleaned) == 0 {
		return nil
	}
	return cleaned
}
