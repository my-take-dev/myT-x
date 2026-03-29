package main

import (
	"myT-x/internal/mcp"
	"myT-x/internal/tmux"
)

// ---------------------------------------------------------------------------
// Wails-bound thin wrappers — delegate to mcpAPIService
// ---------------------------------------------------------------------------

// ListMCPServers returns the MCP snapshot for the given session.
func (a *App) ListMCPServers(sessionName string) ([]mcp.MCPSnapshot, error) {
	svc, err := a.requireMCPAPIService()
	if err != nil {
		return nil, err
	}
	return svc.ListMCPServers(sessionName)
}

// Deprecated: the current MCP manager UI no longer exposes per-MCP toggles.
// This binding is kept for non-UI callers until legacy integrations are removed.
//
// ToggleMCPServer enables or disables an MCP for a session.
func (a *App) ToggleMCPServer(sessionName, mcpID string, enabled bool) error {
	svc, err := a.requireMCPAPIService()
	if err != nil {
		return err
	}
	return svc.ToggleMCPServer(sessionName, mcpID, enabled)
}

// GetMCPDetail returns full detail for one MCP (usage sample, config params, status).
func (a *App) GetMCPDetail(sessionName, mcpID string) (mcp.MCPSnapshot, error) {
	svc, err := a.requireMCPAPIService()
	if err != nil {
		return mcp.MCPSnapshot{}, err
	}
	return svc.GetMCPDetail(sessionName, mcpID)
}

// ResolveMCPStdio resolves a user-facing MCP name, ensures the target MCP is
// enabled for the session, and returns deterministic pipe connection info.
func (a *App) ResolveMCPStdio(sessionName, mcpName string) (tmux.MCPStdioResolution, error) {
	svc, err := a.requireMCPAPIService()
	if err != nil {
		return tmux.MCPStdioResolution{}, err
	}
	return svc.ResolveMCPStdio(sessionName, mcpName)
}
