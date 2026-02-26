package mcp

// Definition describes a built-in MCP server that myT-x can manage.
// Each definition is a static template loaded from config or registered
// programmatically. Runtime state is tracked separately per session in
// InstanceState.
type Definition struct {
	ID             string            `json:"id" yaml:"id"`
	Name           string            `json:"name" yaml:"name"`
	Description    string            `json:"description" yaml:"description"`
	Command        string            `json:"command" yaml:"command"`
	Args           []string          `json:"args,omitempty" yaml:"args,omitempty"`
	DefaultEnv     map[string]string `json:"default_env,omitempty" yaml:"default_env,omitempty"`
	DefaultEnabled bool              `json:"default_enabled" yaml:"default_enabled"`
	UsageSample    string            `json:"usage_sample,omitempty" yaml:"usage_sample,omitempty"`
	ConfigParams   []ConfigParam     `json:"config_params,omitempty" yaml:"config_params,omitempty"`
}

// ConfigParam describes a single user-configurable parameter for an MCP.
type ConfigParam struct {
	Key          string `json:"key" yaml:"key"`
	Label        string `json:"label" yaml:"label"`
	DefaultValue string `json:"default_value" yaml:"default_value"`
	Description  string `json:"description,omitempty" yaml:"description,omitempty"`
}

// Status represents the runtime status of an MCP instance.
type Status string

const (
	StatusStopped  Status = "stopped"
	StatusStarting Status = "starting"
	StatusRunning  Status = "running"
	StatusError    Status = "error"
)

// String returns the string representation of the status.
func (s Status) String() string {
	return string(s)
}

// InstanceState represents the runtime state of an MCP for a specific session.
type InstanceState struct {
	MCPID     string `json:"mcp_id"`
	SessionID string `json:"session_id"`
	Enabled   bool   `json:"enabled"`
	Status    Status `json:"status"`
	// Error is meaningful only when Status == StatusError.
	Error string `json:"error,omitempty"`
}

// Snapshot is the frontend-safe representation that combines the static
// definition with the per-session runtime state. This follows the same
// snapshot pattern as tmux.SessionSnapshot.
type Snapshot struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	Enabled      bool          `json:"enabled"`
	Status       Status        `json:"status"`
	Error        string        `json:"error,omitempty"`
	UsageSample  string        `json:"usage_sample,omitempty"`
	ConfigParams []ConfigParam `json:"config_params,omitempty"`
}

// Backward-compatible aliases.
type MCPDefinition = Definition
type MCPConfigParam = ConfigParam
type MCPStatus = Status
type MCPInstanceState = InstanceState
type MCPSnapshot = Snapshot
