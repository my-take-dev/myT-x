package mcp

import (
	"io"
	"log"
	"os"
	"path/filepath"

	orchestrator "myT-x/internal/mcp/agent-orchestrator"
)

// orchestratorRuntimeFactory creates a RuntimeFactory that produces
// orchestrator.Runtime instances for each pipe connection. The returned
// runtime shares a SQLite database at dbPath for cross-connection state.
func orchestratorRuntimeFactory(dbPath, sessionName string, allPanes bool) RuntimeFactory {
	return func(in io.Reader, out io.Writer) (MCPRuntime, error) {
		return orchestrator.NewRuntime(orchestrator.Config{
			DBPath:          dbPath,
			In:              in,
			Out:             out,
			Logger:          log.New(os.Stderr, "[orchestrator] ", log.LstdFlags),
			SessionName:     sessionName,
			SessionAllPanes: allPanes,
			// SelfResolver is nil → defaults to tmux executor
			// → pipe mode has no TMUX_PANE → trusted mode activates
		})
	}
}

// configParamValue は Definition.ConfigParams から指定キーの DefaultValue を返す。
// 該当キーが存在しない場合は fallback を返す。
func configParamValue(params []ConfigParam, key, fallback string) string {
	for _, p := range params {
		if p.Key == key {
			return p.DefaultValue
		}
	}
	return fallback
}

// buildPipeConfig constructs an MCPPipeConfig for the given definition.
// For orchestrator-kind definitions it uses RuntimeFactory instead of the
// LSP command path.
func buildPipeConfig(pipeName, rootDir, sessionName string, def Definition) MCPPipeConfig {
	switch def.Kind {
	case "orchestrator":
		dbPath := filepath.Join(rootDir, ".myT-x", "orchestrator.db")
		allPanes := configParamValue(def.ConfigParams, "session_all_panes", "false") == "true"
		return MCPPipeConfig{
			PipeName:       pipeName,
			RuntimeFactory: orchestratorRuntimeFactory(dbPath, sessionName, allPanes),
		}
	default: // "" = LSP
		return MCPPipeConfig{
			PipeName:   pipeName,
			LSPCommand: def.Command,
			LSPArgs:    append([]string(nil), def.Args...),
			RootDir:    rootDir,
		}
	}
}
