package config

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

const (
	maxConfigFileBytes int64 = 1 << 20 // 1MB
	maxRenameRetry           = 10
	// Windows file lock releases (antivirus/indexing) typically settle quickly.
	// Use a short linear backoff: baseDelay * (1..maxRenameRetry).
	renameRetryBaseDelay = 10 * time.Millisecond
	minOverrideNameLen   = 5
	// maxValidPort is the highest TCP/UDP port number (2^16 - 1).
	// Port 0 is valid and means "OS auto-assign".
	maxValidPort = 65535
)

// defaultConfigDirFn is a test seam; tests override it to simulate
// directory-resolution failures in validateConfigPath.
var defaultConfigDirFn = defaultConfigDir
var userHomeDirFn = os.UserHomeDir
var windowsEnvTokenPattern = regexp.MustCompile(`%[A-Za-z_][A-Za-z0-9_]*%`)
var posixEnvTokenPattern = regexp.MustCompile(`\$\{[A-Za-z_][A-Za-z0-9_]*\}|\$[A-Za-z_][A-Za-z0-9_]*`)
var yamlUnmarshalConfigMetadataFn = func(raw []byte, out *map[string]any) error {
	return yaml.Unmarshal(raw, out)
}
var defaultPathWarningState struct {
	mu       sync.Mutex
	messages []string
}

func recordDefaultPathWarning(message string) {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return
	}
	defaultPathWarningState.mu.Lock()
	defaultPathWarningState.messages = append(defaultPathWarningState.messages, trimmed)
	defaultPathWarningState.mu.Unlock()
}

// ConsumeDefaultPathWarnings returns and clears path-resolution warnings
// accumulated during DefaultPath() calls.
func ConsumeDefaultPathWarnings() []string {
	defaultPathWarningState.mu.Lock()
	defer defaultPathWarningState.mu.Unlock()
	if len(defaultPathWarningState.messages) == 0 {
		return nil
	}
	out := make([]string, len(defaultPathWarningState.messages))
	copy(out, defaultPathWarningState.messages)
	defaultPathWarningState.messages = nil
	return out
}

// ClaudeEnvConfig holds Claude Code environment variable settings.
// Vars contains key-value pairs applied to terminal panes.
// DefaultEnabled controls the checkbox default in the new session modal.
type ClaudeEnvConfig struct {
	DefaultEnabled bool              `yaml:"default_enabled" json:"default_enabled"`
	Vars           map[string]string `yaml:"vars,omitempty" json:"vars,omitempty"`
}

// Config is myT-x runtime configuration.
type Config struct {
	Shell                 string            `yaml:"shell" json:"shell"`
	Prefix                string            `yaml:"prefix" json:"prefix"`
	Keys                  map[string]string `yaml:"keys" json:"keys"`
	QuakeMode             bool              `yaml:"quake_mode" json:"quake_mode"`
	GlobalHotkey          string            `yaml:"global_hotkey" json:"global_hotkey"`
	Worktree              WorktreeConfig    `yaml:"worktree" json:"worktree"`
	AgentModel            *AgentModel       `yaml:"agent_model,omitempty" json:"agent_model,omitempty"`
	PaneEnv               map[string]string `yaml:"pane_env,omitempty" json:"pane_env,omitempty"`
	PaneEnvDefaultEnabled bool              `yaml:"pane_env_default_enabled" json:"pane_env_default_enabled"`
	ClaudeEnv             *ClaudeEnvConfig  `yaml:"claude_env,omitempty" json:"claude_env,omitempty"`
	// WebSocketPort is the port for the local WebSocket server used for
	// high-throughput pane data streaming. 0 (default) lets the OS assign
	// an available port, which is recommended to avoid port conflicts.
	WebSocketPort int `yaml:"websocket_port" json:"websocket_port"`
	// ViewerShortcuts maps right-sidebar view IDs to keyboard shortcut strings.
	// Example: {"file-tree": "Ctrl+Shift+E", "git-graph": "Ctrl+Shift+G"}
	// When nil or a key is absent, the frontend uses its compiled-in default shortcut.
	ViewerShortcuts map[string]string `yaml:"viewer_shortcuts,omitempty" json:"viewer_shortcuts,omitempty"`
	// DefaultSessionDir is the directory used by Quick Start Session.
	// Empty string means "use the application launch directory".
	DefaultSessionDir string `yaml:"default_session_dir,omitempty" json:"default_session_dir,omitempty"`
	// MCPServers defines built-in MCP server configurations.
	// Each entry describes an MCP that can be toggled per session.
	MCPServers []MCPServerConfig `yaml:"mcp_servers,omitempty" json:"mcp_servers,omitempty"`
}

// MCPServerConfig describes a single MCP server entry in the config file.
type MCPServerConfig struct {
	ID           string                 `yaml:"id" json:"id"`
	Name         string                 `yaml:"name" json:"name"`
	Description  string                 `yaml:"description,omitempty" json:"description,omitempty"`
	Command      string                 `yaml:"command" json:"command"`
	Args         []string               `yaml:"args,omitempty" json:"args,omitempty"`
	Env          map[string]string      `yaml:"env,omitempty" json:"env,omitempty"`
	Enabled      bool                   `yaml:"enabled" json:"enabled"`
	UsageSample  string                 `yaml:"usage_sample,omitempty" json:"usage_sample,omitempty"`
	ConfigParams []MCPServerConfigParam `yaml:"config_params,omitempty" json:"config_params,omitempty"`
}

// MCPServerConfigParam describes a user-configurable parameter for an MCP
// server definition.
type MCPServerConfigParam struct {
	Key          string `yaml:"key" json:"key"`
	Label        string `yaml:"label" json:"label"`
	DefaultValue string `yaml:"default_value" json:"default_value"`
	Description  string `yaml:"description,omitempty" json:"description,omitempty"`
}

// AgentModel holds from-to model name mapping for Agent Teams.
// When both From and To are non-empty, the --model flag in child
// agent commands is replaced: From -> To.
// Overrides take priority: if --agent-name contains an override's Name,
// --model is replaced with that override's Model (first match wins).
//
// Special wildcard: when From is "ALL" (case-insensitive), every --model
// value is replaced with To regardless of its current value. This allows
// blanket model substitution across all child agents. The wildcard check
// is performed by isAllModelFrom() in the tmux-shim model_transform layer.
type AgentModel struct {
	From      string               `yaml:"from" json:"from"`                               // source model name to match (or "ALL" wildcard)
	To        string               `yaml:"to" json:"to"`                                   // replacement model name
	Overrides []AgentModelOverride `yaml:"overrides,omitempty" json:"overrides,omitempty"` // agent-name-based overrides
}

// AgentModelOverride maps an agent-name substring to a specific model.
// When --agent-name contains Name (case-insensitive substring match),
// --model is replaced with Model regardless of its current value.
type AgentModelOverride struct {
	Name  string `yaml:"name" json:"name"`   // substring to match in --agent-name (>= 5 chars)
	Model string `yaml:"model" json:"model"` // model to use for this agent
}

// WorktreeConfig holds worktree-related settings.
// The .wt directory path is auto-computed from repoPath (e.g. /path/to/repo.wt/).
//
// SECURITY: SetupScripts, CopyFiles and CopyDirs are trusted configuration values
// loaded from a protected config file (LOCALAPPDATA/myT-x/config.yaml, 0o600).
// These fields are editable through the settings UI modal, which writes back
// to the same protected config file. This is the intended configuration flow.
// Do NOT expose these fields to untrusted sources (e.g. session metadata from git).
type WorktreeConfig struct {
	Enabled      bool     `yaml:"enabled" json:"enabled"`
	ForceCleanup bool     `yaml:"force_cleanup" json:"force_cleanup"` // Skip uncommitted changes check when removing worktree
	SetupScripts []string `yaml:"setup_scripts" json:"setup_scripts"` // Scripts to run after worktree creation
	CopyFiles    []string `yaml:"copy_files" json:"copy_files"`
	CopyDirs     []string `yaml:"copy_dirs" json:"copy_dirs"` // Directories to recursively copy from repo to worktree
}

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

// DefaultConfig returns default values aligned with spec.
func DefaultConfig() Config {
	return Config{
		Shell:        "powershell.exe",
		Prefix:       "Ctrl+b",
		QuakeMode:    true,
		GlobalHotkey: "Ctrl+Shift+F12",
		Keys: map[string]string{
			"split-vertical":   "%",
			"split-horizontal": "\"",
			"toggle-zoom":      "z",
			"kill-pane":        "x",
			"detach-session":   "d",
		},
		Worktree: WorktreeConfig{
			Enabled:      true,
			SetupScripts: []string{},
			CopyFiles:    []string{},
			CopyDirs:     []string{},
		},
	}
}

// DefaultPath resolves the config file path, preferring LOCALAPPDATA over
// APPDATA, falling back to ~/.config when both are unset, and then to
// os.TempDir() if the home directory cannot be resolved.
// The temp-dir fallback is not a stable persistence location and may vary
// between sessions depending on environment configuration.
func DefaultPath() string {
	base := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if base == "" {
		base = strings.TrimSpace(os.Getenv("APPDATA"))
	}
	if base == "" {
		home, err := userHomeDirFn()
		if err != nil {
			// Keep config path resolvable even in restricted environments.
			slog.Warn("[WARN-CONFIG] using temp dir as config path fallback", "error", err)
			recordDefaultPathWarning(
				"Config path fallback: failed to resolve LOCALAPPDATA/APPDATA/home directory. Using temp directory; settings persistence may be limited.",
			)
			base = os.TempDir()
		} else {
			base = filepath.Join(home, ".config")
		}
	}
	return filepath.Join(base, "myT-x", "config.yaml")
}

// Load reads config file. If file does not exist, defaults are returned.
// The configured shell is validated against an allowlist; an error is returned
// if validation fails.
func Load(path string) (Config, error) {
	cfg := DefaultConfig()
	if path == "" {
		return cfg, errors.New("config path required")
	}

	raw, err := readLimitedFile(path, maxConfigFileBytes)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	if len(raw) == 0 {
		return cfg, nil
	}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		slog.Warn("[WARN-CONFIG] failed to parse config, using defaults", "path", path, "error", err)
		return DefaultConfig(), err
	}

	rawMap, metadataErr := parseRawConfigMetadata(raw)
	defaultWorktreeEnabled := DefaultConfig().Worktree.Enabled
	if metadataErr != nil {
		slog.Warn("[WARN-CONFIG] failed to parse config metadata", "error", metadataErr)
	} else {
		// Warn about deprecated fields that are silently ignored by yaml.Unmarshal.
		warnDeprecatedFields(rawMap)
	}
	hasWorktreeEnabled, resolveErr := resolveWorktreeEnabled(raw, rawMap)
	if resolveErr != nil {
		// Keep already-parsed cfg.Worktree.Enabled to avoid silently overwriting
		// explicit user values when helper probing is unavailable.
		slog.Warn("[WARN-CONFIG] failed to resolve worktree.enabled metadata, preserving parsed value", "error", resolveErr)
	} else if !hasWorktreeEnabled {
		cfg.Worktree.Enabled = defaultWorktreeEnabled
	}
	if err := applyDefaultsAndValidate(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// EnsureFile writes default config if missing and returns loaded config.
func EnsureFile(path string) (Config, error) {
	cfg, err := Load(path)
	if err != nil {
		return cfg, err
	}
	if _, statErr := os.Stat(path); errors.Is(statErr, os.ErrNotExist) {
		if _, err := Save(path, cfg); err != nil {
			return cfg, err
		}
	}
	return cfg, nil
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

// MinOverrideNameLen returns the minimum allowed rune length for
// agent_model.overrides[*].name.
func MinOverrideNameLen() int {
	return minOverrideNameLen
}

// Clone returns a deep copy of cfg.
// Use this when sharing config snapshots across goroutines or package boundaries.
func Clone(src Config) Config {
	dst := src

	if src.Keys != nil {
		dst.Keys = make(map[string]string, len(src.Keys))
		maps.Copy(dst.Keys, src.Keys)
	}

	dst.Worktree.SetupScripts = cloneStringSlice(src.Worktree.SetupScripts)
	dst.Worktree.CopyFiles = cloneStringSlice(src.Worktree.CopyFiles)
	dst.Worktree.CopyDirs = cloneStringSlice(src.Worktree.CopyDirs)

	if src.AgentModel != nil {
		agentModelCopy := *src.AgentModel
		agentModelCopy.Overrides = cloneAgentModelOverrides(src.AgentModel.Overrides)
		dst.AgentModel = &agentModelCopy
	}

	if src.PaneEnv != nil {
		dst.PaneEnv = make(map[string]string, len(src.PaneEnv))
		maps.Copy(dst.PaneEnv, src.PaneEnv)
	}

	if src.ClaudeEnv != nil {
		claudeEnvCopy := *src.ClaudeEnv
		if src.ClaudeEnv.Vars != nil {
			claudeEnvCopy.Vars = make(map[string]string, len(src.ClaudeEnv.Vars))
			maps.Copy(claudeEnvCopy.Vars, src.ClaudeEnv.Vars)
		}
		dst.ClaudeEnv = &claudeEnvCopy
	}

	if src.ViewerShortcuts != nil {
		dst.ViewerShortcuts = make(map[string]string, len(src.ViewerShortcuts))
		maps.Copy(dst.ViewerShortcuts, src.ViewerShortcuts)
	}

	if src.MCPServers != nil {
		dst.MCPServers = make([]MCPServerConfig, len(src.MCPServers))
		for i, s := range src.MCPServers {
			dst.MCPServers[i] = s
			if s.Args != nil {
				dst.MCPServers[i].Args = cloneStringSlice(s.Args)
			}
			if s.Env != nil {
				dst.MCPServers[i].Env = make(map[string]string, len(s.Env))
				maps.Copy(dst.MCPServers[i].Env, s.Env)
			}
			if s.ConfigParams != nil {
				dst.MCPServers[i].ConfigParams = cloneMCPServerConfigParams(s.ConfigParams)
			}
		}
	}

	return dst
}

func cloneStringSlice(src []string) []string {
	if src == nil {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

func cloneAgentModelOverrides(src []AgentModelOverride) []AgentModelOverride {
	if src == nil {
		return nil
	}
	dst := make([]AgentModelOverride, len(src))
	copy(dst, src)
	return dst
}

func cloneMCPServerConfigParams(src []MCPServerConfigParam) []MCPServerConfigParam {
	if src == nil {
		return nil
	}
	dst := make([]MCPServerConfigParam, len(src))
	copy(dst, src)
	return dst
}

// Save validates cfg, fills defaults, and atomically writes to path.
// Returns the normalized config that was actually written to disk.
// Uses the same validation rules as Load (shell allowlist, agent model constraints).
func Save(path string, cfg Config) (Config, error) {
	normalizedPath, err := validateConfigPath(path)
	if err != nil {
		return cfg, err
	}
	if err := applyDefaultsAndValidate(&cfg); err != nil {
		return cfg, fmt.Errorf("save config: %w", err)
	}

	raw, err := yaml.Marshal(cfg)
	if err != nil {
		return cfg, fmt.Errorf("save config: marshal: %w", err)
	}
	if err := atomicWrite(normalizedPath, raw); err != nil {
		return cfg, err
	}
	slog.Debug("[DEBUG-CONFIG] config saved", "path", path)
	return cfg, nil
}

// atomicWrite writes config data using temp-file + rename to avoid partial
// writes and retries rename on Windows to tolerate transient file locks.
func atomicWrite(path string, data []byte) (err error) {
	dir := filepath.Dir(path)
	if err = os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("save config: mkdir: %w", err)
	}

	// Atomic write: temp file + rename in same directory ensures
	// same-filesystem rename and prevents partial writes on crash.
	tmpFile, err := os.CreateTemp(dir, ".config.yaml.tmp.*")
	if err != nil {
		return fmt.Errorf("save config: create temp: %w", err)
	}
	tmpPath := tmpFile.Name()

	defer func() {
		if tmpFile != nil {
			if closeErr := tmpFile.Close(); closeErr != nil && !errors.Is(closeErr, os.ErrClosed) {
				slog.Warn("[WARN-CONFIG] failed to close temp file", "path", tmpPath, "error", closeErr)
			}
		}
		if err != nil {
			if removeErr := os.Remove(tmpPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				slog.Warn("[WARN-CONFIG] failed to remove temp file", "path", tmpPath, "error", removeErr)
			}
		}
	}()

	if err = tmpFile.Chmod(0o600); err != nil {
		return fmt.Errorf("save config: chmod temp: %w", err)
	}
	if _, err = tmpFile.Write(data); err != nil {
		return fmt.Errorf("save config: write: %w", err)
	}
	if err = tmpFile.Sync(); err != nil {
		return fmt.Errorf("save config: sync: %w", err)
	}
	err = tmpFile.Close()
	tmpFile = nil
	if err != nil {
		return fmt.Errorf("save config: close: %w", err)
	}

	if err = renameFileWithRetry(tmpPath, path); err != nil {
		return fmt.Errorf("save config: rename: %w", err)
	}
	return nil
}

// validateConfigPath normalizes path and enforces that config writes stay
// inside the default config directory when that directory is resolvable.
func validateConfigPath(path string) (string, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return "", errors.New("config path required")
	}
	absolutePath, err := filepath.Abs(trimmedPath)
	if err != nil {
		return "", fmt.Errorf("save config: resolve path: %w", err)
	}

	expectedDir, err := defaultConfigDirFn()
	if err != nil {
		return "", fmt.Errorf("save config: resolve config dir: %w", err)
	}
	absoluteExpectedDir, err := filepath.Abs(expectedDir)
	if err != nil {
		return "", fmt.Errorf("save config: resolve config dir: %w", err)
	}
	if !pathWithinDir(absolutePath, absoluteExpectedDir) {
		return "", fmt.Errorf("save config: path outside config directory: %q", absolutePath)
	}

	return absolutePath, nil
}

func defaultConfigDir() (string, error) {
	return filepath.Dir(DefaultPath()), nil
}

// pathWithinDir blocks directory traversal by ensuring path is under dir.
// It also rejects Windows cross-drive escapes because filepath.Rel returns
// an absolute path when roots differ.
func pathWithinDir(path string, dir string) bool {
	relativePath, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(path))
	if err != nil {
		return false
	}
	if relativePath == "." {
		return true
	}
	if relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(os.PathSeparator)) {
		return false
	}
	return !filepath.IsAbs(relativePath)
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

// validateDefaultSessionDir normalizes DefaultSessionDir in place.
// Expands ~ prefix to the user's home directory, applies filepath.Clean,
// and clears non-absolute paths with a warning log (non-fatal).
func validateDefaultSessionDir(cfg *Config) {
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

func expandDefaultSessionDirEnv(dir string) string {
	if dir == "" {
		return ""
	}
	// Expand Windows-style %VAR% tokens on all platforms for portability.
	expanded := windowsEnvTokenPattern.ReplaceAllStringFunc(dir, func(token string) string {
		key := token[1 : len(token)-1]
		if value, ok := os.LookupEnv(key); ok {
			return value
		}
		return token
	})
	// Skip POSIX-style $VAR expansion on Windows: '$' is a valid character
	// in Windows file paths (e.g. C:\Users\foo$bar) and should not be
	// interpreted as an environment variable reference.
	if runtime.GOOS == "windows" {
		return expanded
	}
	expanded = posixEnvTokenPattern.ReplaceAllStringFunc(expanded, func(token string) string {
		key := strings.TrimPrefix(token, "$")
		key = strings.TrimPrefix(key, "{")
		key = strings.TrimSuffix(key, "}")
		if value, ok := os.LookupEnv(key); ok {
			return value
		}
		return token
	})
	return expanded
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

// maxCustomEnvValueBytes is the downstream limit enforced by CommandRouter.
// Config layer warns early for values exceeding this threshold.
// Why 8192: matches tmux.maxCustomEnvValueBytes for early user feedback.
const maxCustomEnvValueBytes = 8192

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

// parseRawConfigMetadata unmarshals raw YAML into a generic map used only
// for metadata checks (deprecated fields and missing option detection).
func parseRawConfigMetadata(raw []byte) (map[string]any, error) {
	var rawMap map[string]any
	if err := yamlUnmarshalConfigMetadataFn(raw, &rawMap); err != nil {
		return nil, err
	}
	return rawMap, nil
}

type rawWorktreeEnabledProbe struct {
	Worktree *struct {
		Enabled *bool `yaml:"enabled"`
	} `yaml:"worktree"`
}

func probeRawWorktreeEnabled(raw []byte) (bool, error) {
	var probe rawWorktreeEnabledProbe
	if err := yaml.Unmarshal(raw, &probe); err != nil {
		return false, err
	}
	if probe.Worktree == nil {
		return false, nil
	}
	return probe.Worktree.Enabled != nil, nil
}

func resolveWorktreeEnabled(raw []byte, rawMap map[string]any) (bool, error) {
	if rawMap != nil {
		wt, ok := rawMap["worktree"].(map[string]any)
		if !ok {
			return false, nil
		}
		_, hasEnabled := wt["enabled"]
		return hasEnabled, nil
	}
	return probeRawWorktreeEnabled(raw)
}

func warnDeprecatedFields(rawMap map[string]any) {
	wt, ok := rawMap["worktree"].(map[string]any)
	if !ok {
		return
	}
	if _, has := wt["auto_cleanup"]; has {
		slog.Warn("[WARN-CONFIG] deprecated field ignored: worktree.auto_cleanup is no longer used")
	}
}

func readLimitedFile(path string, maxBytes int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	limited := io.LimitReader(file, maxBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > maxBytes {
		return nil, fmt.Errorf("config file exceeds %d bytes", maxBytes)
	}
	return raw, nil
}

func isZeroConfig(cfg Config) bool {
	// reflect.DeepEqual guards against field-addition drift that manual checks miss.
	return reflect.DeepEqual(cfg, Config{})
}

func renameFileWithRetry(sourcePath string, targetPath string) error {
	var lastErr error
	for attempt := range maxRenameRetry {
		err := os.Rename(sourcePath, targetPath)
		if err == nil {
			return nil
		}
		lastErr = err
		if runtime.GOOS != "windows" {
			return err
		}
		time.Sleep(time.Duration(attempt+1) * renameRetryBaseDelay)
	}
	return lastErr
}
