package config

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
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
	if strings.TrimSpace(cfg.ViewerSidebarMode) == "" {
		cfg.ViewerSidebarMode = defaults.ViewerSidebarMode
	}
	if cfg.Worktree.SetupScripts == nil {
		cfg.Worktree.SetupScripts = append([]string(nil), defaults.Worktree.SetupScripts...)
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
	sanitizePaneEnv(cfg)
	sanitizeClaudeEnv(cfg)
	sanitizeMCPServers(cfg)
	validateDefaultSessionDir(cfg)
	return nil
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
// range (30-95). Zero means "use default" (80).
func validateChatOverlayPercentage(cfg *Config) {
	if cfg.ChatOverlayPercentage == 0 {
		cfg.ChatOverlayPercentage = 80
	}
	if cfg.ChatOverlayPercentage < 30 {
		slog.Warn("[WARN-CONFIG] chat_overlay_percentage too low, clamping to 30",
			"configured", cfg.ChatOverlayPercentage)
		cfg.ChatOverlayPercentage = 30
	}
	if cfg.ChatOverlayPercentage > 95 {
		slog.Warn("[WARN-CONFIG] chat_overlay_percentage too high, clamping to 95",
			"configured", cfg.ChatOverlayPercentage)
		cfg.ChatOverlayPercentage = 95
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

	baseName := strings.ToLower(filepath.Base(shell))
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
