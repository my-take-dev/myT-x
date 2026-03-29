package config

import "reflect"

const (
	// minOverrideNameLen is the minimum rune length for agent_model.overrides[*].name.
	minOverrideNameLen = 5
)

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
	// ViewerSidebarMode controls how the right viewer sidebar is rendered.
	// Empty string keeps backward-compatible overlay behavior.
	// "overlay" is the explicit default value written by DefaultConfig.
	// "docked" renders the viewer beside the main content.
	ViewerSidebarMode string `yaml:"viewer_sidebar_mode,omitempty" json:"viewer_sidebar_mode,omitempty"`
	// DefaultSessionDir is the directory used by Quick Start Session.
	// Empty string means "use the application launch directory".
	DefaultSessionDir string `yaml:"default_session_dir,omitempty" json:"default_session_dir,omitempty"`
	// MCPServers defines built-in MCP server configurations.
	// Each entry describes an MCP that can be toggled per session.
	MCPServers []MCPServerConfig `yaml:"mcp_servers,omitempty" json:"mcp_servers,omitempty"`
	// ChatOverlayPercentage controls the height of the expanded chat overlay
	// as a percentage of the terminal area (30-95). Default is 80.
	ChatOverlayPercentage int `yaml:"chat_overlay_percentage,omitempty" json:"chat_overlay_percentage,omitempty"`
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
		ViewerSidebarMode:     "overlay",
		ChatOverlayPercentage: 80,
	}
}

// MinOverrideNameLen returns the minimum allowed rune length for
// agent_model.overrides[*].name.
func MinOverrideNameLen() int {
	return minOverrideNameLen
}

func isZeroConfig(cfg Config) bool {
	// reflect.DeepEqual guards against field-addition drift that manual checks miss.
	return reflect.DeepEqual(cfg, Config{})
}
