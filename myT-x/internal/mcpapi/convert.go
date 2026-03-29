package mcpapi

import (
	"log/slog"
	"maps"
	"os"
	"strings"

	"myT-x/internal/config"
	"myT-x/internal/mcp"
	"myT-x/internal/mcp/lspmcp/lsppkg"
)

// DefaultBridgeCommand is the fallback executable name used for stdio bridge
// launch recommendations when os.Executable cannot be resolved.
const DefaultBridgeCommand = "myT-x.exe"

// ResolveBridgeCommand returns the absolute path to the current executable
// for use as the stdio bridge command. Falls back to DefaultBridgeCommand
// on error or empty path.
func ResolveBridgeCommand() string {
	return resolveBridgeCommandWith(os.Executable)
}

// resolveBridgeCommandWith is the testable core of ResolveBridgeCommand,
// allowing tests to inject test doubles for os.Executable.
func resolveBridgeCommandWith(executableFn func() (string, error)) string {
	exePath, err := executableFn()
	if err != nil {
		slog.Debug("[DEBUG-MCP] os.Executable failed, using fallback bridge command", "error", err)
		return DefaultBridgeCommand
	}
	if trimmedPath := strings.TrimSpace(exePath); trimmedPath != "" {
		return trimmedPath
	}
	slog.Debug("[DEBUG-MCP] os.Executable returned empty path, using fallback bridge command")
	return DefaultBridgeCommand
}

// MCPServerConfigsToDefinitions converts config MCPServerConfig entries to
// mcp.Definition entries for registry loading.
func MCPServerConfigsToDefinitions(configs []config.MCPServerConfig) []mcp.Definition {
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

// LSPExtensionMetaToDefinitions converts lsppkg.ExtensionMeta entries to
// mcp.Definition entries for registry loading. All auto-registered entries
// have DefaultEnabled=false.
func LSPExtensionMetaToDefinitions(metas []lsppkg.ExtensionMeta) []mcp.Definition {
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
