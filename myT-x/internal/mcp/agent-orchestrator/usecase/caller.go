package usecase

import (
	"context"
	"errors"
	"log"
	"time"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

const trustedCallerName = "_trusted"

// IsTrustedCaller returns true when the agent represents a pipe-bridge caller
// that bypassed pane-based authentication.
func IsTrustedCaller(agent domain.Agent) bool {
	return agent.Name == trustedCallerName
}

// resolveCaller resolves the caller pane and enforces that it belongs to a registered agent.
// This is the authorization entry point shared by all MCP tools.
//
// When running behind MCPPipeServer (pipe bridge mode), TMUX_PANE is unavailable
// because all connections share the parent process environment. In that case the
// resolver returns an error or an empty pane ID and the caller is treated as
// trusted — ACL checks are bypassed for inter-agent coordination on the same machine.
func resolveCaller(ctx context.Context, resolver domain.SelfPaneResolver, repo domain.AgentRepository, logger *log.Logger) (domain.Agent, error) {
	paneID, err := resolver.GetPaneID(ctx)
	if err != nil || paneID == "" {
		// Pipe bridge mode: TMUX_PANE unavailable. Trust the caller.
		logf(logger, "caller pane unavailable (pipe bridge mode): treating as trusted")
		return domain.Agent{Name: trustedCallerName}, nil
	}
	logf(logger, "resolved caller pane_id=%s", paneID)
	agent, err := repo.GetAgentByPaneID(ctx, paneID)
	if err != nil {
		logf(logger, "caller is not registered: %v", err)
		return domain.Agent{}, errors.New("caller is not registered")
	}
	logf(logger, "resolved caller agent=%s pane_id=%s", agent.Name, agent.PaneID)
	return agent, nil
}

func ensureLogger(logger *log.Logger) *log.Logger {
	if logger == nil {
		return log.Default()
	}
	return logger
}

func logf(logger *log.Logger, format string, args ...any) {
	if logger != nil {
		logger.Printf(format, args...)
	}
}

func operationError(logger *log.Logger, public string, err error) error {
	logf(logger, "%s: %v", public, err)
	return errors.New(public)
}

// sleepContext はコンテキストキャンセルに対応した sleep。
func sleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
