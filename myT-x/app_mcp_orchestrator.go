package main

import "myT-x/internal/mcp"

// orchestratorMCPDefinitions returns the built-in agent-orchestrator MCP
// definition. It is registered alongside LSP definitions at startup.
func orchestratorMCPDefinitions() []mcp.Definition {
	return []mcp.Definition{{
		ID:             "orch-agent-orchestrator",
		Name:           "Agent Orchestrator",
		Description:    "tmux上の複数AIエージェント間でタスク送信・返信・状態管理を行うMCPサーバー",
		Kind:           "orchestrator",
		DefaultEnabled: false,
	}}
}
