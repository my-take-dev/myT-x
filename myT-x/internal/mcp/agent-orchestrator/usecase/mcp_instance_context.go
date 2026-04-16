package usecase

import (
	"context"
	"log"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

type mcpInstanceIDContextKey struct{}

// WithMCPInstanceID binds the current MCP runtime instance to the use case context.
func WithMCPInstanceID(ctx context.Context, instanceID string) context.Context {
	if instanceID == "" {
		return ctx
	}
	return context.WithValue(ctx, mcpInstanceIDContextKey{}, instanceID)
}

// logTrustedCallerInstanceHint emits a debug log mapping the current MCP
// runtime instance to a registered agent, when the instance ID is known.
//
// This function is observability-only. Callers MUST discard the returned agent
// and MUST NOT use it for authorization decisions. The name deliberately
// avoids "recover" / "resolve" wording (its predecessor was named
// recoverTrustedCallerByInstance, which was misread as "feeds authorization"
// by reviewers — see /ACCEPTED_DESIGN_DECISIONS.md AD-002).
//
// The return value is kept to preserve the existing error-logging shape at
// call sites; there is no intent to consume it. If a future change needs
// instance-based authorization, the design decision AD-002 must be revised
// in the same PR.
func logTrustedCallerInstanceHint(ctx context.Context, repo domain.AgentRepository, logger *log.Logger) (domain.Agent, error) {
	instanceID, ok := currentMCPInstanceID(ctx)
	if !ok {
		logf(logger, "trusted caller hint failed: missing mcp instance id")
		return domain.Agent{}, errCallerNotRegistered
	}

	agent, err := repo.GetAgentByMCPInstanceID(ctx, instanceID)
	if err != nil {
		logf(logger, "trusted caller hint failed for instance_id=%s: %v", instanceID, err)
		return domain.Agent{}, errCallerNotRegistered
	}

	logf(logger, "trusted caller hint matched instance_id=%s agent=%s pane_id=%s", instanceID, agent.Name, agent.PaneID)
	return agent, nil
}

func currentMCPInstanceID(ctx context.Context) (string, bool) {
	instanceID, ok := ctx.Value(mcpInstanceIDContextKey{}).(string)
	if !ok || instanceID == "" {
		return "", false
	}
	return instanceID, true
}
