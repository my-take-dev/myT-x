package mcpapi

import (
	"path/filepath"
	"slices"
	"strings"
)

// genericMCPLaunchers are intentionally excluded from aliasing to avoid
// high-collision names like "node", "go", "python", or "npx".
var genericMCPLaunchers = map[string]struct{}{
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
