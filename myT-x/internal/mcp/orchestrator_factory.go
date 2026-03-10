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
func orchestratorRuntimeFactory(dbPath string) RuntimeFactory {
	return func(in io.Reader, out io.Writer) (MCPRuntime, error) {
		return orchestrator.NewRuntime(orchestrator.Config{
			DBPath: dbPath,
			In:     in,
			Out:    out,
			Logger: log.New(os.Stderr, "[orchestrator] ", log.LstdFlags),
			// SelfResolver is nil → defaults to tmux executor
			// → pipe mode has no TMUX_PANE → trusted mode activates
		})
	}
}

// buildPipeConfig constructs an MCPPipeConfig for the given definition.
// For orchestrator-kind definitions it uses RuntimeFactory instead of the
// LSP command path.
func buildPipeConfig(pipeName, rootDir string, def Definition) MCPPipeConfig {
	switch def.Kind {
	case "orchestrator":
		dbPath := filepath.Join(rootDir, ".myT-x", "orchestrator.db")
		return MCPPipeConfig{
			PipeName:       pipeName,
			RuntimeFactory: orchestratorRuntimeFactory(dbPath),
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
