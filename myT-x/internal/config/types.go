package config

// ClaudeEnvConfig holds Claude Code environment variable settings.
// Vars contains key-value pairs applied to terminal panes.
// DefaultEnabled controls the checkbox default in the new session modal.
type ClaudeEnvConfig struct {
	DefaultEnabled bool              `yaml:"default_enabled" json:"default_enabled"`
	Vars           map[string]string `yaml:"vars,omitempty" json:"vars,omitempty"`
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
