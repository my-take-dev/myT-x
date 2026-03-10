package tmux

import "myT-x/internal/mcp/agent-orchestrator/domain"

// ValidatePaneID は domain.ValidatePaneID に委譲する。
func ValidatePaneID(paneID string) error {
	return domain.ValidatePaneID(paneID)
}
