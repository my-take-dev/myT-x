package main

import (
	"fmt"
	"log/slog"
	"strings"

	"myT-x/internal/config"
	"myT-x/internal/mcp"
)

// ListMCPServers returns the MCP snapshot for the given session.
// Each snapshot contains the static definition merged with the per-session
// runtime state (enabled/disabled, status).
func (a *App) ListMCPServers(sessionName string) ([]mcp.MCPSnapshot, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		err := fmt.Errorf("session name is required")
		slog.Warn("[WARN-MCP] list mcp servers failed", "session", sessionName, "error", err)
		return nil, fmt.Errorf("list mcp servers: %w", err)
	}
	mgr, err := a.requireMCPManager()
	if err != nil {
		slog.Warn("[WARN-MCP] list mcp servers failed", "session", sessionName, "error", err)
		return nil, fmt.Errorf("list mcp servers: %w", err)
	}
	snapshots, err := mgr.SnapshotForSession(sessionName)
	if err != nil {
		slog.Warn("[WARN-MCP] list mcp servers failed", "session", sessionName, "error", err)
		return nil, fmt.Errorf("list mcp servers: %w", err)
	}
	return snapshots, nil
}

// ToggleMCPServer enables or disables an MCP for a session.
func (a *App) ToggleMCPServer(sessionName, mcpID string, enabled bool) error {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		err := fmt.Errorf("session name is required")
		slog.Warn("[WARN-MCP] toggle mcp server failed", "session", sessionName, "mcpID", mcpID, "enabled", enabled, "error", err)
		return fmt.Errorf("toggle mcp server: %w", err)
	}
	mcpID = strings.TrimSpace(mcpID)
	if mcpID == "" {
		err := fmt.Errorf("mcp ID is required")
		slog.Warn("[WARN-MCP] toggle mcp server failed", "session", sessionName, "mcpID", mcpID, "enabled", enabled, "error", err)
		return fmt.Errorf("toggle mcp server: %w", err)
	}
	mgr, err := a.requireMCPManager()
	if err != nil {
		slog.Warn("[WARN-MCP] toggle mcp server failed", "session", sessionName, "mcpID", mcpID, "enabled", enabled, "error", err)
		return fmt.Errorf("toggle mcp server: %w", err)
	}
	if err := mgr.SetEnabled(sessionName, mcpID, enabled); err != nil {
		slog.Warn("[WARN-MCP] toggle mcp server failed", "session", sessionName, "mcpID", mcpID, "enabled", enabled, "error", err)
		return fmt.Errorf("toggle mcp server: %w", err)
	}
	return nil
}

// GetMCPDetail returns full detail for one MCP (usage sample, config params, status).
func (a *App) GetMCPDetail(sessionName, mcpID string) (mcp.MCPSnapshot, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		err := fmt.Errorf("session name is required")
		slog.Warn("[WARN-MCP] get mcp detail failed", "session", sessionName, "mcpID", mcpID, "error", err)
		return mcp.MCPSnapshot{}, fmt.Errorf("get mcp detail: %w", err)
	}
	mcpID = strings.TrimSpace(mcpID)
	if mcpID == "" {
		err := fmt.Errorf("mcp ID is required")
		slog.Warn("[WARN-MCP] get mcp detail failed", "session", sessionName, "mcpID", mcpID, "error", err)
		return mcp.MCPSnapshot{}, fmt.Errorf("get mcp detail: %w", err)
	}
	mgr, err := a.requireMCPManager()
	if err != nil {
		slog.Warn("[WARN-MCP] get mcp detail failed", "session", sessionName, "mcpID", mcpID, "error", err)
		return mcp.MCPSnapshot{}, fmt.Errorf("get mcp detail: %w", err)
	}
	detail, err := mgr.GetDetail(sessionName, mcpID)
	if err != nil {
		slog.Warn("[WARN-MCP] get mcp detail failed", "session", sessionName, "mcpID", mcpID, "error", err)
		return mcp.MCPSnapshot{}, fmt.Errorf("get mcp detail: %w", err)
	}
	return detail, nil
}

// mcpServerConfigsToDefinitions converts config MCPServerConfig entries to
// mcp.Definition entries for registry loading.
func mcpServerConfigsToDefinitions(configs []config.MCPServerConfig) []mcp.Definition {
	if len(configs) == 0 {
		return nil
	}
	defs := make([]mcp.Definition, 0, len(configs))
	for _, c := range configs {
		def := mcp.Definition{
			ID:          c.ID,
			Name:        c.Name,
			Description: c.Description,
			Command:     c.Command,
			Args:        cloneMCPConfigArgs(c.Args),
			// Config field "env" is mapped to runtime definition field "default_env".
			DefaultEnv:     cloneMCPConfigEnv(c.Env),
			DefaultEnabled: c.Enabled,
			UsageSample:    c.UsageSample,
			ConfigParams:   cloneMCPConfigParams(c.ConfigParams),
		}
		defs = append(defs, def)
	}
	return defs
}

func cloneMCPConfigArgs(src []string) []string {
	if src == nil {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

func cloneMCPConfigEnv(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func cloneMCPConfigParams(src []config.MCPServerConfigParam) []mcp.ConfigParam {
	if src == nil {
		return nil
	}
	dst := make([]mcp.ConfigParam, len(src))
	for i, p := range src {
		dst[i] = mcp.ConfigParam{
			Key:          p.Key,
			Label:        p.Label,
			DefaultValue: p.DefaultValue,
			Description:  p.Description,
		}
	}
	return dst
}
