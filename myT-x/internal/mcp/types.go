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
	// Kind distinguishes MCP server types for startInstance branching.
	Kind DefinitionKind `json:"kind,omitempty" yaml:"kind,omitempty"`
}

// DefinitionKind identifies the type of MCP server.
type DefinitionKind string

const (
	// DefinitionKindLSP is the default command-backed MCP kind. An omitted or
	// empty YAML "kind" decodes to this value; unknown non-empty kinds stay
	// command-backed via the external process path.
	DefinitionKindLSP DefinitionKind = ""
	// DefinitionKindCustom is the explicit command-backed kind used by
	// config-defined external MCP servers.
	DefinitionKindCustom DefinitionKind = "custom"
	// DefinitionKindOrchestrator is the Agent Orchestrator MCP server.
	DefinitionKindOrchestrator DefinitionKind = "orchestrator"
	// DefinitionKindSingleTaskRunner is the Single Task Runner MCP server.
	DefinitionKindSingleTaskRunner DefinitionKind = "single-task-runner"
)

// IsBuiltIn reports whether the kind is one of the backend-defined symbolic
// kinds. Custom config-defined kinds are allowed and are command-backed.
func (k DefinitionKind) IsBuiltIn() bool {
	switch k {
	case DefinitionKindLSP, DefinitionKindOrchestrator, DefinitionKindSingleTaskRunner:
		return true
	default:
		return false
	}
}

// UsesEmbeddedRuntime reports whether the kind is handled by a backend-owned
// runtime factory instead of an external command.
func (k DefinitionKind) UsesEmbeddedRuntime() bool {
	switch k {
	case DefinitionKindOrchestrator, DefinitionKindSingleTaskRunner:
		return true
	default:
		return false
	}
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
	// PipePath is the Named Pipe path when the MCP instance is running.
	// Empty when the MCP instance is not running.
	PipePath string `json:"pipe_path,omitempty"`
	// BridgeCommand is the stdio bridge executable path used by CLI clients.
	// Empty when no bridge launch recommendation is available.
	BridgeCommand string `json:"bridge_command,omitempty"`
	// BridgeArgs contains arguments for BridgeCommand when a bridge launch
	// recommendation is available.
	BridgeArgs []string `json:"bridge_args,omitempty"`
	// Kind distinguishes MCP server types for frontend category rendering.
	// See DefinitionKind constants for possible values.
	Kind DefinitionKind `json:"kind,omitempty"`
}

// Backward-compatible aliases.
type MCPDefinition = Definition
type MCPConfigParam = ConfigParam
type MCPStatus = Status
type MCPInstanceState = InstanceState
type MCPSnapshot = Snapshot
