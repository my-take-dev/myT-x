package main

import (
	"myT-x/internal/mcp"
	"myT-x/internal/singletaskrunner"
)

// singleTaskRunnerMCPDefinitions returns the built-in single-task-runner MCP definition.
func singleTaskRunnerMCPDefinitions() []mcp.Definition {
	return []mcp.Definition{{
		ID:             "single-task-runner",
		Name:           "Single Task Runner",
		Description:    "Sequentially dispatches queued tasks to their configured target panes and waits for explicit " + singletaskrunner.ResolutionToolNames + " MCP tool calls.",
		Kind:           mcp.DefinitionKindSingleTaskRunner,
		DefaultEnabled: false,
	}}
}
