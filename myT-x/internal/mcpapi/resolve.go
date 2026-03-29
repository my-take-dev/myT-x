package mcpapi

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"myT-x/internal/mcp"
	"myT-x/internal/tmux"
)

// Default readiness wait parameters used by ResolveMCPStdio.
const (
	defaultReadinessWaitTimeout  = 5 * time.Second
	defaultReadinessWaitInterval = 100 * time.Millisecond
)

// ResolveMCPStdio resolves a user-facing MCP name (e.g. "gopls"), ensures
// the target MCP is enabled for the session, and returns deterministic pipe
// connection info for stdio clients.
//
// If the MCP was previously disabled and an error occurs after enabling, the
// enabled state is rolled back.
//
// The pipe name is deterministic per session/MCP pair, so callers do not need
// this API to busy-wait for the runtime to reach StatusRunning. The CLI bridge
// already performs a bounded dial with timeout, which is the correct place to
// wait for listener readiness without blocking the app IPC handler.
func (s *Service) ResolveMCPStdio(sessionName, mcpName string) (tmux.MCPStdioResolution, error) {
	var mcpID string
	fail := func(err error) (tmux.MCPStdioResolution, error) {
		return tmux.MCPStdioResolution{}, logResolveMCPStdioFailure(sessionName, mcpName, mcpID, err)
	}

	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return fail(fmt.Errorf("session name is required"))
	}
	mcpName = strings.TrimSpace(mcpName)
	if mcpName == "" {
		return fail(fmt.Errorf("mcp name is required"))
	}

	mgr, err := s.deps.RequireMCPManager()
	if err != nil {
		return fail(err)
	}
	mcpID, err = s.resolveMCPIDForCLIName(mcpName)
	if err != nil {
		return fail(err)
	}
	slog.Debug("[DEBUG-MCP] resolved mcp cli name",
		"session", sessionName,
		"input", mcpName,
		"mcpID", mcpID,
	)
	initialDetail, err := mgr.GetDetail(sessionName, mcpID)
	if err != nil {
		return fail(fmt.Errorf("get mcp detail %q before enabling: %w", mcpID, err))
	}
	rollbackNeeded := !initialDetail.Enabled

	if err := mgr.SetEnabled(sessionName, mcpID, true); err != nil {
		return fail(fmt.Errorf("enable mcp %q: %w", mcpID, err))
	}
	if rollbackNeeded {
		slog.Debug("[DEBUG-MCP] auto-enabled mcp for stdio resolution",
			"session", sessionName,
			"mcpID", mcpID,
		)
	}

	detail, err := mgr.GetDetail(sessionName, mcpID)
	if err != nil {
		return fail(rollbackResolvedMCP(
			mgr,
			sessionName,
			mcpID,
			rollbackNeeded,
			fmt.Errorf("get mcp detail %q: %w", mcpID, err),
		))
	}
	if !detail.Enabled {
		return fail(rollbackResolvedMCP(
			mgr,
			sessionName,
			mcpID,
			rollbackNeeded,
			fmt.Errorf("mcp %q is not enabled", mcpID),
		))
	}

	// Wait for StatusStarting to transition before returning to reduce CLI
	// dial retry pressure. The CLI bridge still retries as a safety net.
	if detail.Status == mcp.StatusStarting {
		waitTimeout := s.deps.ReadinessWaitTimeout
		waitInterval := s.deps.ReadinessWaitInterval
		waitStart := time.Now()
		slog.Debug("[DEBUG-MCP] resolve mcp stdio: waiting for mcp to become ready",
			"session", sessionName,
			"mcpID", mcpID,
			"waitTimeout", waitTimeout,
		)
		deadline := time.Now().Add(waitTimeout)
		for detail.Status == mcp.StatusStarting && time.Now().Before(deadline) {
			time.Sleep(waitInterval)
			detail, err = mgr.GetDetail(sessionName, mcpID)
			if err != nil {
				return fail(rollbackResolvedMCP(
					mgr, sessionName, mcpID, rollbackNeeded,
					fmt.Errorf("get mcp detail %q during readiness wait: %w", mcpID, err),
				))
			}
		}
		slog.Debug("[DEBUG-MCP] resolve mcp stdio: readiness wait completed",
			"session", sessionName,
			"mcpID", mcpID,
			"finalStatus", detail.Status,
			"waitDuration", time.Since(waitStart),
		)
	}

	switch detail.Status {
	case mcp.StatusRunning:
	case mcp.StatusStarting:
		// StatusStarting is still allowed after readiness wait timeout.
		// The CLI bridge performs bounded retry for listener readiness.
		slog.Debug("[DEBUG-MCP] resolve mcp stdio: mcp is still starting after readiness wait; cli bridge will dial with retry",
			"session", sessionName,
			"mcpID", mcpID,
		)
	case mcp.StatusError:
		msg := strings.TrimSpace(detail.Error)
		if msg == "" {
			msg = "unknown startup error"
		}
		return fail(rollbackResolvedMCP(
			mgr,
			sessionName,
			mcpID,
			rollbackNeeded,
			fmt.Errorf("mcp %q failed to start: %s", mcpID, msg),
		))
	case mcp.StatusStopped:
		return fail(rollbackResolvedMCP(
			mgr,
			sessionName,
			mcpID,
			rollbackNeeded,
			fmt.Errorf("mcp %q was stopped before becoming ready", mcpID),
		))
	default:
		return fail(rollbackResolvedMCP(
			mgr,
			sessionName,
			mcpID,
			rollbackNeeded,
			fmt.Errorf("mcp %q entered unexpected status %q", mcpID, detail.Status),
		))
	}

	pipePath := strings.TrimSpace(detail.PipePath)
	if pipePath == "" {
		pipePath = mcp.BuildMCPPipeName(sessionName, mcpID)
		slog.Debug("[DEBUG-MCP] resolve mcp stdio: using deterministic pipe fallback",
			"session", sessionName,
			"mcpID", mcpID,
			"pipePath", pipePath,
		)
	}
	slog.Debug("[DEBUG-MCP] resolve mcp stdio succeeded",
		"session", sessionName,
		"mcpID", mcpID,
		"pipePath", pipePath,
		"status", detail.Status,
	)
	return tmux.MCPStdioResolution{
		SessionName: sessionName,
		MCPID:       mcpID,
		PipePath:    pipePath,
	}, nil
}

func rollbackResolvedMCP(mgr *mcp.Manager, sessionName, mcpID string, rollback bool, cause error) error {
	if mgr == nil || !rollback {
		return cause
	}
	slog.Debug("[DEBUG-MCP] rolling back auto-enabled mcp",
		"session", sessionName,
		"mcpID", mcpID,
		"cause", cause,
	)
	if rollbackErr := mgr.SetEnabled(sessionName, mcpID, false); rollbackErr != nil {
		slog.Warn("[WARN-MCP] resolve mcp stdio rollback failed",
			"session", sessionName,
			"mcpID", mcpID,
			"error", rollbackErr,
			"cause", cause,
		)
		return errors.Join(cause, fmt.Errorf("rollback failed: %w", rollbackErr))
	}
	return cause
}

func logResolveMCPStdioFailure(sessionName, mcpName, mcpID string, err error) error {
	slog.Warn("[WARN-MCP] resolve mcp stdio failed",
		"session", sessionName,
		"mcpName", mcpName,
		"mcpID", mcpID,
		"error", err,
	)
	return err
}

func (s *Service) resolveMCPIDForCLIName(input string) (string, error) {
	name := strings.TrimSpace(input)
	if name == "" {
		return "", fmt.Errorf("mcp name is required")
	}
	registry, err := s.deps.RequireMCPRegistry()
	if err != nil {
		return "", err
	}

	defs := registry.All()
	if len(defs) == 0 {
		return "", fmt.Errorf("no mcp definitions are registered")
	}

	aliasToIDs := make(map[string]map[string]struct{}, len(defs)*2)
	for _, def := range defs {
		if strings.EqualFold(def.ID, name) {
			return def.ID, nil
		}
		addMCPAlias(aliasToIDs, def.ID, def.ID)
		addMCPAlias(aliasToIDs, def.Name, def.ID)
		if strings.HasPrefix(strings.ToLower(def.ID), "lsp-") {
			addMCPAlias(aliasToIDs, strings.TrimSpace(def.ID[4:]), def.ID)
		}
		if strings.HasPrefix(strings.ToLower(def.ID), "orch-") {
			addMCPAlias(aliasToIDs, strings.TrimSpace(def.ID[5:]), def.ID)
		}
		if cmdAlias := normalizeMCPAliasToken(def.Command); cmdAlias != "" {
			if _, excluded := genericMCPLaunchers[cmdAlias]; !excluded {
				addMCPAlias(aliasToIDs, cmdAlias, def.ID)
			}
		}
	}

	normalizedInput := normalizeMCPAliasToken(name)
	candidates := sortedAliasCandidates(aliasToIDs[normalizedInput])
	switch len(candidates) {
	case 1:
		return candidates[0], nil
	case 0:
		var aliases []string
		for alias := range aliasToIDs {
			aliases = append(aliases, alias)
		}
		slices.Sort(aliases)
		if len(aliases) > 8 {
			aliases = aliases[:8]
		}
		return "", fmt.Errorf("unknown mcp %q (examples: %s)", input, strings.Join(aliases, ", "))
	default:
		return "", fmt.Errorf("mcp %q is ambiguous: %s", input, strings.Join(candidates, ", "))
	}
}
