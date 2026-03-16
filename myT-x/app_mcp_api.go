package main

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"myT-x/internal/config"
	"myT-x/internal/mcp"
	"myT-x/internal/mcp/lspmcp/lsppkg"
	"myT-x/internal/tmux"
)

var (
	osExecutableFn = os.Executable
	// Generic launchers are intentionally excluded from aliasing to avoid
	// high-collision names like "node", "go", "python", or "npx".
	genericMCPLaunchers = map[string]struct{}{
		"bash":       {},
		"bun":        {},
		"cargo":      {},
		"cmd":        {},
		"deno":       {},
		"dotnet":     {},
		"go":         {},
		"java":       {},
		"node":       {},
		"nodejs":     {},
		"npm":        {},
		"npx":        {},
		"perl":       {},
		"php":        {},
		"pnpm":       {},
		"powershell": {},
		"pwsh":       {},
		"py":         {},
		"python":     {},
		"python3":    {},
		"ruby":       {},
		"sh":         {},
		"uv":         {},
		"uvx":        {},
		"yarn":       {},
	}
)

const defaultMCPBridgeCommand = "myT-x.exe"

func resolveMCPBridgeCommand() string {
	exePath, err := osExecutableFn()
	if err != nil {
		slog.Debug("[DEBUG-MCP] os.Executable failed, using fallback bridge command", "error", err)
		return defaultMCPBridgeCommand
	}
	if trimmedPath := strings.TrimSpace(exePath); trimmedPath != "" {
		return trimmedPath
	}
	slog.Debug("[DEBUG-MCP] os.Executable returned empty path, using fallback bridge command")
	return defaultMCPBridgeCommand
}

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
	for i := range snapshots {
		a.applyMCPBridgeRecommendation(sessionName, &snapshots[i])
	}
	return snapshots, nil
}

// Deprecated: the current MCP manager UI no longer exposes per-MCP toggles.
// This binding is kept for non-UI callers until legacy integrations are removed.
//
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
	a.applyMCPBridgeRecommendation(sessionName, &detail)
	return detail, nil
}

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
func (a *App) ResolveMCPStdio(sessionName, mcpName string) (tmux.MCPStdioResolution, error) {
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

	mgr, err := a.requireMCPManager()
	if err != nil {
		return fail(err)
	}
	mcpID, err = a.resolveMCPIDForCLIName(mcpName)
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
	switch detail.Status {
	case mcp.StatusRunning:
	case mcp.StatusStarting:
		// StatusStarting is intentionally allowed here. The pipe path is
		// deterministic, and the CLI bridge performs the bounded wait for
		// listener readiness instead of blocking this IPC call.
		slog.Debug("[DEBUG-MCP] resolve mcp stdio: mcp is still starting; cli bridge will dial with timeout",
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

// applyMCPBridgeRecommendation populates bridge launch metadata on a snapshot.
// Recommendations are provided for every LSP and orchestrator MCP regardless
// of runtime status because the CLI bridge performs a bounded dial with timeout
// and can be shown as ready-to-copy guidance even before the MCP reaches
// running state.
func (a *App) applyMCPBridgeRecommendation(sessionName string, snapshot *mcp.MCPSnapshot) {
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
		return
	}

	command := strings.TrimSpace(a.mcpBridgeCommand)
	if command == "" {
		command = defaultMCPBridgeCommand
	}
	snapshot.BridgeCommand = command
	snapshot.BridgeArgs = []string{
		"mcp",
		"stdio",
		"--mcp",
		mcpName,
	}
}

func (a *App) resolveMCPIDForCLIName(input string) (string, error) {
	name := strings.TrimSpace(input)
	if name == "" {
		return "", fmt.Errorf("mcp name is required")
	}
	registry, err := a.requireMCPRegistry()
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

func addMCPAlias(aliasToIDs map[string]map[string]struct{}, alias, mcpID string) {
	normalizedAlias := normalizeMCPAliasToken(alias)
	if normalizedAlias == "" || strings.TrimSpace(mcpID) == "" {
		return
	}
	ids := aliasToIDs[normalizedAlias]
	if ids == nil {
		ids = make(map[string]struct{}, 1)
		aliasToIDs[normalizedAlias] = ids
	}
	ids[mcpID] = struct{}{}
}

func sortedAliasCandidates(idSet map[string]struct{}) []string {
	if len(idSet) == 0 {
		return nil
	}
	candidates := make([]string, 0, len(idSet))
	for id := range idSet {
		candidates = append(candidates, id)
	}
	slices.Sort(candidates)
	return candidates
}

func normalizeMCPAliasToken(value string) string {
	trimmed := strings.Trim(strings.TrimSpace(value), `"'`)
	if trimmed == "" {
		return ""
	}
	base := strings.ToLower(filepath.Base(trimmed))
	for {
		next := base
		next = strings.TrimSuffix(next, ".exe")
		next = strings.TrimSuffix(next, ".cmd")
		next = strings.TrimSuffix(next, ".bat")
		// Strip Windows executable suffixes for command-style aliases such as
		// "cmd.com.exe". Exact MCP ID matches are resolved before alias
		// normalization, so this only affects alias-style lookups.
		next = strings.TrimSuffix(next, ".com")
		if next == base {
			break
		}
		base = next
	}
	return strings.TrimSpace(base)
}

// orchMCPCLINameFromID extracts the CLI-facing MCP name from an orchestrator MCP ID.
// Returns "" for non-orchestrator IDs and invalid IDs such as "orch-".
// Example: "orch-agent-orchestrator" -> "agent-orchestrator", "lsp-gopls" -> "".
func orchMCPCLINameFromID(mcpID string) string {
	trimmed := strings.TrimSpace(mcpID)
	const prefix = "orch-"
	if len(trimmed) <= len(prefix) || !strings.EqualFold(trimmed[:len(prefix)], prefix) {
		return ""
	}
	normalized := normalizeMCPAliasToken(trimmed[len(prefix):])
	if normalized != "" {
		return normalized
	}
	return strings.TrimSpace(trimmed[len(prefix):])
}

// lspMCPCLINameFromID extracts the CLI-facing MCP name from an LSP MCP ID.
// Returns "" for non-LSP IDs and invalid LSP IDs such as "lsp-".
// Example: "lsp-gopls" -> "gopls", "memory" -> "".
func lspMCPCLINameFromID(mcpID string) string {
	trimmed := strings.TrimSpace(mcpID)
	const prefix = "lsp-"
	if len(trimmed) <= len(prefix) || !strings.EqualFold(trimmed[:len(prefix)], prefix) {
		return ""
	}
	normalized := normalizeMCPAliasToken(trimmed[len(prefix):])
	if normalized != "" {
		return normalized
	}
	return strings.TrimSpace(trimmed[len(prefix):])
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

// lspExtensionMetaToDefinitions converts lsppkg.ExtensionMeta entries to
// mcp.Definition entries for registry loading. All auto-registered entries
// have DefaultEnabled=false.
func lspExtensionMetaToDefinitions(metas []lsppkg.ExtensionMeta) []mcp.Definition {
	if len(metas) == 0 {
		return nil
	}
	defs := make([]mcp.Definition, 0, len(metas))
	for _, m := range metas {
		defs = append(defs, mcp.Definition{
			ID:             "lsp-" + m.Name,
			Name:           m.Language + " (LSP: " + m.Name + ")",
			Description:    m.Language + " language server via " + m.DefaultCommand,
			Command:        m.DefaultCommand,
			DefaultEnabled: false,
		})
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
	maps.Copy(dst, src)
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
