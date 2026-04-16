package usecase

import (
	"context"
	"log"
	"log/slog"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

func preflightTaskCaller(
	ctx context.Context,
	resolver domain.SelfPaneResolver,
	agents domain.AgentRepository,
	tasks domain.TaskRepository,
	logger *log.Logger,
) (domain.Agent, error) {
	caller, err := resolveCaller(ctx, resolver, agents, logger)
	if err != nil {
		return domain.Agent{}, err
	}
	if err := expirePendingTasks(ctx, tasks, logger); err != nil {
		slog.Warn("[WARN-MCP-ORCH] preflight: expire pending tasks failed", "error", err)
	}
	return caller, nil
}

func preflightAssigneeTaskCaller(
	ctx context.Context,
	resolver domain.SelfPaneResolver,
	agents domain.AgentRepository,
	tasks domain.TaskRepository,
	logger *log.Logger,
) (domain.Agent, error) {
	caller, err := preflightTaskCaller(ctx, resolver, agents, tasks, logger)
	if err != nil {
		return domain.Agent{}, err
	}
	if !IsTrustedCaller(caller) {
		return caller, nil
	}

	// Observability-only: emits a debug log mapping the MCP instance to an
	// agent when possible. The returned agent is intentionally discarded —
	// trusted callers are NOT narrowed to the recovered agent; they remain
	// authorized for any assignee (see /ACCEPTED_DESIGN_DECISIONS.md AD-002).
	if _, err := logTrustedCallerInstanceHint(ctx, agents, logger); err != nil {
		logf(logger, "trusted caller assignee hint unavailable: %v", err)
	}
	return caller, nil
}

func preflightAssigneeTaskAgentCaller(
	ctx context.Context,
	resolver domain.SelfPaneResolver,
	agents domain.AgentRepository,
	tasks domain.TaskRepository,
	logger *log.Logger,
	agentName string,
) (domain.Agent, error) {
	caller, err := preflightAssigneeTaskCaller(ctx, resolver, agents, tasks, logger)
	if err != nil {
		return domain.Agent{}, err
	}
	if IsTrustedCaller(caller) {
		return caller, nil
	}
	if caller.Name != agentName {
		return domain.Agent{}, accessDeniedError("caller does not match agent_name")
	}
	return caller, nil
}
