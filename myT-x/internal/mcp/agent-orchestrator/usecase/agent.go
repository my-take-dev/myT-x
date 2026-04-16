package usecase

import (
	"context"
	"log"
	"strings"
	"time"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

const orchestratorAgentName = "orchestrator"

// RegisterAgentCmd はエージェント登録コマンド。
type RegisterAgentCmd struct {
	Name          string
	PaneID        string
	Role          string
	Skills        []domain.Skill
	MCPInstanceID string
}

// RegisterAgentResult はエージェント登録結果。
type RegisterAgentResult struct {
	Name         string
	PaneID       string
	Role         string
	Skills       []domain.Skill
	PaneTitle    string
	TitleWarning string
}

// ListAgentsResult はエージェント一覧結果。
type ListAgentsResult struct {
	Agents       []AgentEntry
	Unregistered []string
	Orchestrator *OrchestratorEntry
	Warning      string
}

// AgentEntry はエージェント一覧のエントリ。
type AgentEntry struct {
	Name   string
	PaneID string
	Role   string
	Skills []domain.Skill
	Status domain.AgentWorkStatus
}

// OrchestratorEntry はオーケストレーター情報。
type OrchestratorEntry struct {
	PaneID string
}

// AgentService はエージェントの登録・一覧を管理する。
type AgentService struct {
	agents      domain.AgentRepository
	statuses    domain.AgentStatusRepository
	resolver    domain.SelfPaneResolver
	lister      domain.PaneLister
	titleSetter domain.PaneTitleSetter
	logger      *log.Logger
}

// NewAgentService は AgentService を構築する。
func NewAgentService(
	agents domain.AgentRepository,
	statuses domain.AgentStatusRepository,
	resolver domain.SelfPaneResolver,
	lister domain.PaneLister,
	titleSetter domain.PaneTitleSetter,
	logger *log.Logger,
) *AgentService {
	return &AgentService{
		agents:      agents,
		statuses:    statuses,
		resolver:    resolver,
		lister:      lister,
		titleSetter: titleSetter,
		logger:      ensureLogger(logger),
	}
}

// Register はエージェントを登録する。
func (s *AgentService) Register(ctx context.Context, cmd RegisterAgentCmd) (RegisterAgentResult, error) {
	if err := domain.ValidatePaneID(cmd.PaneID); err != nil {
		return RegisterAgentResult{}, validationError(err.Error())
	}

	agent := domain.Agent{
		Name:          cmd.Name,
		PaneID:        cmd.PaneID,
		Role:          cmd.Role,
		Skills:        cmd.Skills,
		MCPInstanceID: cmd.MCPInstanceID,
	}
	var defaultStatus *domain.AgentStatus
	if s.statuses != nil {
		defaultStatus = &domain.AgentStatus{
			AgentName: cmd.Name,
			Status:    domain.AgentWorkStatusIdle,
			UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		}
	}
	if err := s.agents.ReplaceAgentRegistration(ctx, agent, defaultStatus); err != nil {
		return RegisterAgentResult{}, operationError(s.logger, "failed to register agent", err)
	}

	title := cmd.Name
	if cmd.Role != "" {
		title = cmd.Name + ":" + cmd.Role
	}
	if cmd.Name == orchestratorAgentName {
		title = orchestratorAgentName
	}

	result := RegisterAgentResult{
		Name:      cmd.Name,
		PaneID:    cmd.PaneID,
		Role:      cmd.Role,
		Skills:    cmd.Skills,
		PaneTitle: title,
	}
	if err := s.titleSetter.SetPaneTitle(ctx, cmd.PaneID, title); err != nil {
		logf(s.logger, "set pane title for %s (%s): %v", cmd.Name, cmd.PaneID, err)
		result.TitleWarning = "pane title update failed"
	}

	return result, nil
}

// List はエージェント一覧を取得する。
func (s *AgentService) List(ctx context.Context) (ListAgentsResult, error) {
	caller, err := resolveCaller(ctx, s.resolver, s.agents, s.logger)
	if err != nil {
		return ListAgentsResult{}, err
	}

	agents, err := s.agents.ListAgents(ctx)
	if err != nil {
		return ListAgentsResult{}, operationError(s.logger, "failed to list agents", err)
	}
	var statuses []domain.AgentStatus
	statusWarning := ""
	if s.statuses != nil {
		statuses, err = s.statuses.ListAgentStatuses(ctx)
		if err != nil {
			logf(s.logger, "list agent statuses: %v", err)
			statusWarning = "failed to inspect agent statuses; status may be unknown"
			statuses = nil
		}
	}
	statusByAgent := make(map[string]domain.AgentWorkStatus, len(statuses))
	for _, status := range statuses {
		statusByAgent[status.AgentName] = status.Status
	}

	registeredPaneIDs := make(map[string]bool)
	var result ListAgentsResult
	warnings := make([]string, 0, 4)
	panes, err := s.lister.ListPanes(ctx)
	paneExists := make(map[string]struct{}, len(panes))
	invalidPaneAgents := make(map[string]struct{})
	removedStalePaneIDs := make(map[string]struct{})
	failedStalePaneIDs := make(map[string]struct{})
	if err != nil {
		logf(s.logger, "list panes: %v", err)
		warnings = append(warnings, "failed to inspect tmux panes; unregistered_panes may be incomplete")
	} else {
		for _, pane := range panes {
			paneExists[pane.ID] = struct{}{}
		}
		if shouldSkipStaleAgentCleanup(caller, agents, paneExists) {
			warnings = append(warnings, "skipped stale agent cleanup because tmux pane inspection was incomplete")
		} else {
			for _, agent := range agents {
				if agent.PaneID == "" {
					logf(s.logger, "list_agents: skip cleanup for agent %s with empty pane_id", agent.Name)
					invalidPaneAgents[agent.Name] = struct{}{}
					continue
				}
				if domain.IsVirtualPaneID(agent.PaneID) {
					continue
				}
				if _, ok := paneExists[agent.PaneID]; ok {
					continue
				}
				if err := s.agents.DeleteAgentsByPaneID(ctx, agent.PaneID); err != nil {
					logf(s.logger, "list_agents: cleanup stale pane %s for agent %s: %v", agent.PaneID, agent.Name, err)
					failedStalePaneIDs[agent.PaneID] = struct{}{}
					continue
				}
				removedStalePaneIDs[agent.PaneID] = struct{}{}
			}
		}
		for _, agent := range agents {
			if agent.PaneID == "" {
				if _, recorded := invalidPaneAgents[agent.Name]; recorded {
					continue
				}
				logf(s.logger, "list_agents: skip cleanup for agent %s with empty pane_id", agent.Name)
				invalidPaneAgents[agent.Name] = struct{}{}
			}
		}
	}
	if statusWarning != "" {
		warnings = append(warnings, statusWarning)
	}
	if len(removedStalePaneIDs) > 0 {
		warnings = append(warnings, "removed stale agent registrations for missing panes")
	}
	if len(invalidPaneAgents) > 0 {
		warnings = append(warnings, "found agent registrations with empty pane_id")
	}
	if len(failedStalePaneIDs) > 0 {
		warnings = append(warnings, "failed to remove stale agent registrations for missing panes")
	}

	for _, a := range agents {
		if _, removed := removedStalePaneIDs[a.PaneID]; removed {
			continue
		}
		if a.PaneID != "" {
			registeredPaneIDs[a.PaneID] = true
		}
		if a.Name == orchestratorAgentName {
			result.Orchestrator = &OrchestratorEntry{PaneID: a.PaneID}
		} else {
			agentStatus := statusByAgent[a.Name]
			if agentStatus == "" {
				if _, invalid := invalidPaneAgents[a.Name]; invalid {
					agentStatus = domain.AgentWorkStatusUnknown
				} else if _, stale := failedStalePaneIDs[a.PaneID]; stale {
					agentStatus = domain.AgentWorkStatusUnknown
				} else if statusWarning != "" {
					agentStatus = domain.AgentWorkStatusUnknown
				} else {
					agentStatus = domain.AgentWorkStatusIdle
				}
			}
			result.Agents = append(result.Agents, AgentEntry{
				Name:   a.Name,
				PaneID: a.PaneID,
				Role:   a.Role,
				Skills: a.Skills,
				Status: agentStatus,
			})
		}
	}

	for _, p := range panes {
		if !registeredPaneIDs[p.ID] {
			result.Unregistered = append(result.Unregistered, p.ID)
		}
	}
	if len(warnings) > 0 {
		result.Warning = strings.Join(warnings, "; ")
	}

	return result, nil
}

func shouldSkipStaleAgentCleanup(caller domain.Agent, agents []domain.Agent, paneExists map[string]struct{}) bool {
	if len(paneExists) == 0 {
		for _, agent := range agents {
			if agent.PaneID == "" || domain.IsVirtualPaneID(agent.PaneID) {
				continue
			}
			return true
		}
	}
	if caller.PaneID != "" && !domain.IsVirtualPaneID(caller.PaneID) {
		if _, ok := paneExists[caller.PaneID]; !ok {
			return true
		}
	}
	return false
}
