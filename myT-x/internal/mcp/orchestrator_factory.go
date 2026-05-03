package mcp

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	orchestrator "myT-x/internal/mcp/agent-orchestrator"
	"myT-x/internal/orchestratorstorage"
	"myT-x/internal/singletaskrunner"
	"myT-x/internal/tmux"
)

// orchestratorRuntimeFactory creates a RuntimeFactory that produces
// orchestrator.Runtime instances for each pipe connection. The returned
// runtime shares a SQLite database at dbPath for cross-connection state.
func orchestratorRuntimeFactory(
	dbPath string,
	projectRoot string,
	sessionName string,
	allPanes bool,
	router *tmux.CommandRouter,
	emitFn func(string, any),
) RuntimeFactory {
	return func(in io.Reader, out io.Writer) (MCPRuntime, error) {
		cfg := orchestrator.Config{
			DBPath:          dbPath,
			ProjectRoot:     projectRoot,
			In:              in,
			Out:             out,
			Logger:          log.New(os.Stderr, "[orchestrator] ", log.LstdFlags),
			SessionName:     sessionName,
			SessionAllPanes: allPanes,
			EmitFn:          emitFn,
			// SelfResolver is nil → defaults to tmux executor
			// → pipe mode has no TMUX_PANE → trusted mode activates
		}
		if router != nil {
			cfg.Splitter = &routerBackedSplitter{router: router}
		}
		return orchestrator.NewRuntime(cfg)
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
	configDir               func() (string, error)
	sessionName             string
	router                  *tmux.CommandRouter
	emitFn                  func(string, any)
	singleTaskRunnerManager *singletaskrunner.ServiceManager
}

type routerBackedSplitter struct {
	router *tmux.CommandRouter
}

func (r *routerBackedSplitter) SplitPane(_ context.Context, targetPaneID string, horizontal bool) (string, error) {
	if r == nil || r.router == nil {
		return "", fmt.Errorf("router-backed splitter requires a router")
	}
	return r.router.SplitWindowInternal(targetPaneID, horizontal)
}

// buildPipeConfig constructs an MCPPipeConfig for the given definition.
// For orchestrator-kind and single-task-runner-kind definitions it uses
// RuntimeFactory instead of the external command path. All other kinds,
// including custom config-defined kinds, use the external command path.
func buildPipeConfig(pipeName string, def Definition, ctx pipeConfigContext) (MCPPipeConfig, error) {
	switch def.Kind {
	case DefinitionKindOrchestrator:
		if ctx.configDir == nil {
			return MCPPipeConfig{}, fmt.Errorf("config dir provider is required for orchestrator pipe %s", pipeName)
		}
		configDir, err := ctx.configDir()
		if err != nil {
			return MCPPipeConfig{}, fmt.Errorf("resolve config dir for orchestrator pipe %s: %w", pipeName, err)
		}
		dbPath, err := orchestratorstorage.DBPath(configDir, ctx.rootDir)
		if err != nil {
			return MCPPipeConfig{}, fmt.Errorf("resolve orchestrator db path for pipe %s: %w", pipeName, err)
		}
		allPanes := configParamValue(def.ConfigParams, "session_all_panes", "false") == "true"
		return MCPPipeConfig{
			PipeName:       pipeName,
			RuntimeFactory: orchestratorRuntimeFactory(dbPath, ctx.rootDir, ctx.sessionName, allPanes, ctx.router, ctx.emitFn),
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
