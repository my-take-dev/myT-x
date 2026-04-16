package mcp

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	orchestrator "myT-x/internal/mcp/agent-orchestrator"
	"myT-x/internal/singletaskrunner"
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

type pipeConfigContext struct {
	rootDir                 string
	sessionName             string
	singleTaskRunnerManager *singletaskrunner.ServiceManager
}

// buildPipeConfig constructs an MCPPipeConfig for the given definition.
// For orchestrator-kind and single-task-runner-kind definitions it uses
// RuntimeFactory instead of the external command path. All other kinds,
// including custom config-defined kinds, use the external command path.
func buildPipeConfig(pipeName string, def Definition, ctx pipeConfigContext) (MCPPipeConfig, error) {
	switch def.Kind {
	case DefinitionKindOrchestrator:
		dbPath := filepath.Join(ctx.rootDir, ".myT-x", "orchestrator.db")
		allPanes := configParamValue(def.ConfigParams, "session_all_panes", "false") == "true"
		return MCPPipeConfig{
			PipeName:       pipeName,
			RuntimeFactory: orchestratorRuntimeFactory(dbPath, ctx.sessionName, allPanes),
		}, nil
	case DefinitionKindSingleTaskRunner:
		if ctx.singleTaskRunnerManager == nil {
			return MCPPipeConfig{}, fmt.Errorf("single-task-runner manager is required for pipe %s", pipeName)
		}
		return MCPPipeConfig{
			PipeName:       pipeName,
			RuntimeFactory: singleTaskRunnerRuntimeFactory(ctx.sessionName, ctx.singleTaskRunnerManager),
		}, nil
	default:
		return MCPPipeConfig{
			PipeName:   pipeName,
			LSPCommand: def.Command,
			LSPArgs:    append([]string(nil), def.Args...),
			RootDir:    ctx.rootDir,
		}, nil
	}
}
