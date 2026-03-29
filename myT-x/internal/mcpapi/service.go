package mcpapi

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"myT-x/internal/mcp"
)

// Deps holds external dependencies injected into the mcpapi Service.
type Deps struct {
	// RequireMCPManager returns the MCP Manager or an error if not initialized.
	// Required: must be non-nil.
	RequireMCPManager func() (*mcp.Manager, error)

	// RequireMCPRegistry returns the MCP Registry or an error if not initialized.
	// Required: must be non-nil.
	RequireMCPRegistry func() (*mcp.Registry, error)

	// BridgeCommand returns the resolved path to the myT-x executable used
	// for stdio bridge launch recommendations. Returns "" if not yet resolved.
	// Optional: defaults to func() string { return DefaultBridgeCommand }.
	BridgeCommand func() string

	// ReadinessWaitTimeout is the maximum time to wait for an MCP to become
	// ready during ResolveMCPStdio. Must be > 0. Defaults to 5s.
	ReadinessWaitTimeout time.Duration

	// ReadinessWaitInterval is the polling interval for MCP readiness checks.
	// Must be > 0. Defaults to 100ms.
	ReadinessWaitInterval time.Duration
}

// Service provides MCP API operations extracted from the App layer.
// All state lives in mcp.Manager and mcp.Registry; this service is stateless.
type Service struct {
	deps Deps
}

// NewService creates a new mcpapi Service. Panics if required dependencies
// are nil (fail-fast for programming errors during construction).
func NewService(deps Deps) *Service {
	if deps.RequireMCPManager == nil {
		panic("mcpapi.NewService: RequireMCPManager must be non-nil")
	}
	if deps.RequireMCPRegistry == nil {
		panic("mcpapi.NewService: RequireMCPRegistry must be non-nil")
	}
	if deps.BridgeCommand == nil {
		deps.BridgeCommand = func() string { return DefaultBridgeCommand }
	}
	if deps.ReadinessWaitTimeout == 0 {
		deps.ReadinessWaitTimeout = defaultReadinessWaitTimeout
	}
	if deps.ReadinessWaitInterval == 0 {
		deps.ReadinessWaitInterval = defaultReadinessWaitInterval
	}
	return &Service{deps: deps}
}

// logAndWrapError logs an MCP API failure and returns a wrapped error.
// Consolidates the repeated slog.Warn + fmt.Errorf pattern across API methods.
func logAndWrapError(operation string, err error, attrs ...any) error {
	args := append([]any{"error", err}, attrs...)
	slog.Warn("[WARN-MCP] "+operation+" failed", args...)
	return fmt.Errorf("%s: %w", operation, err)
}

// validateRequired trims a string parameter in place and returns an error if
// the result is empty. Used to consolidate repeated TrimSpace + empty-check
// validation across API methods.
func validateRequired(value *string, name string) error {
	*value = strings.TrimSpace(*value)
	if *value == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

// ListMCPServers returns the MCP snapshot for the given session.
// Each snapshot contains the static definition merged with the per-session
// runtime state (enabled/disabled, status).
func (s *Service) ListMCPServers(sessionName string) ([]mcp.MCPSnapshot, error) {
	fail := func(err error) ([]mcp.MCPSnapshot, error) {
		return nil, logAndWrapError("list mcp servers", err, "session", sessionName)
	}
	if err := validateRequired(&sessionName, "session name"); err != nil {
		return fail(err)
	}
	mgr, err := s.deps.RequireMCPManager()
	if err != nil {
		return fail(err)
	}
	snapshots, err := mgr.SnapshotForSession(sessionName)
	if err != nil {
		return fail(err)
	}
	for i := range snapshots {
		s.applyBridgeRecommendation(sessionName, &snapshots[i])
	}
	return snapshots, nil
}

// Deprecated: the current MCP manager UI no longer exposes per-MCP toggles.
// This binding is kept for non-UI callers until legacy integrations are removed.
//
// ToggleMCPServer enables or disables an MCP for a session.
func (s *Service) ToggleMCPServer(sessionName, mcpID string, enabled bool) error {
	fail := func(err error) error {
		return logAndWrapError("toggle mcp server", err, "session", sessionName, "mcpID", mcpID, "enabled", enabled)
	}
	if err := validateRequired(&sessionName, "session name"); err != nil {
		return fail(err)
	}
	if err := validateRequired(&mcpID, "mcp ID"); err != nil {
		return fail(err)
	}
	mgr, err := s.deps.RequireMCPManager()
	if err != nil {
		return fail(err)
	}
	if err := mgr.SetEnabled(sessionName, mcpID, enabled); err != nil {
		return fail(err)
	}
	return nil
}

// GetMCPDetail returns full detail for one MCP (usage sample, config params, status).
func (s *Service) GetMCPDetail(sessionName, mcpID string) (mcp.MCPSnapshot, error) {
	fail := func(err error) (mcp.MCPSnapshot, error) {
		return mcp.MCPSnapshot{}, logAndWrapError("get mcp detail", err, "session", sessionName, "mcpID", mcpID)
	}
	if err := validateRequired(&sessionName, "session name"); err != nil {
		return fail(err)
	}
	if err := validateRequired(&mcpID, "mcp ID"); err != nil {
		return fail(err)
	}
	mgr, err := s.deps.RequireMCPManager()
	if err != nil {
		return fail(err)
	}
	detail, err := mgr.GetDetail(sessionName, mcpID)
	if err != nil {
		return fail(err)
	}
	s.applyBridgeRecommendation(sessionName, &detail)
	return detail, nil
}

// applyBridgeRecommendation populates bridge launch metadata on a snapshot.
// Recommendations are provided for every LSP and orchestrator MCP regardless
// of runtime status because the CLI bridge performs a bounded dial with timeout
// and can be shown as ready-to-copy guidance even before the MCP reaches
// running state.
func (s *Service) applyBridgeRecommendation(sessionName string, snapshot *mcp.MCPSnapshot) {
	if snapshot == nil {
		return
	}
	// Always clear recommendation fields first to avoid stale values.
	snapshot.BridgeCommand = ""
	snapshot.BridgeArgs = nil

	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return
	}
	mcpName := strings.TrimSpace(lspMCPCLINameFromID(snapshot.ID))
	if mcpName == "" {
		mcpName = strings.TrimSpace(orchMCPCLINameFromID(snapshot.ID))
	}
	if mcpName == "" {
		// Custom MCPs without lsp-/orch- prefix do not support bridge recommendations.
		return
	}

	command := strings.TrimSpace(s.deps.BridgeCommand())
	if command == "" {
		command = DefaultBridgeCommand
	}
	snapshot.BridgeCommand = command
	snapshot.BridgeArgs = []string{
		"mcp",
		"stdio",
		"--mcp",
		mcpName,
	}
}
