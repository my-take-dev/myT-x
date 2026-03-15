package usecase

import (
	"context"
	"errors"
	"log"

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
}

// OrchestratorEntry はオーケストレーター情報。
type OrchestratorEntry struct {
	PaneID string
}

// AgentService はエージェントの登録・一覧を管理する。
type AgentService struct {
	agents      domain.AgentRepository
	resolver    domain.SelfPaneResolver
	lister      domain.PaneLister
	titleSetter domain.PaneTitleSetter
	logger      *log.Logger
}

// NewAgentService は AgentService を構築する。
func NewAgentService(
	agents domain.AgentRepository,
	resolver domain.SelfPaneResolver,
	lister domain.PaneLister,
	titleSetter domain.PaneTitleSetter,
	logger *log.Logger,
) *AgentService {
	return &AgentService{
		agents:      agents,
		resolver:    resolver,
		lister:      lister,
		titleSetter: titleSetter,
		logger:      ensureLogger(logger),
	}
}

// Register はエージェントを登録する。
func (s *AgentService) Register(ctx context.Context, cmd RegisterAgentCmd) (RegisterAgentResult, error) {
	if _, err := s.agents.GetAgent(ctx, cmd.Name); err != nil && !errors.Is(err, domain.ErrNotFound) {
		return RegisterAgentResult{}, operationError(s.logger, "failed to check existing agent registration", err)
	}

	// Keep pane_id -> agent resolution unambiguous for pane-based lookups.
	if err := s.agents.DeleteAgentsByPaneID(ctx, cmd.PaneID); err != nil {
		return RegisterAgentResult{}, operationError(s.logger, "failed to replace existing pane registration", err)
	}

	agent := domain.Agent{
		Name:          cmd.Name,
		PaneID:        cmd.PaneID,
		Role:          cmd.Role,
		Skills:        cmd.Skills,
		MCPInstanceID: cmd.MCPInstanceID,
	}
	if err := s.agents.UpsertAgent(ctx, agent); err != nil {
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
	if _, err := resolveCaller(ctx, s.resolver, s.agents, s.logger); err != nil {
		return ListAgentsResult{}, err
	}

	agents, err := s.agents.ListAgents(ctx)
	if err != nil {
		return ListAgentsResult{}, operationError(s.logger, "failed to list agents", err)
	}

	registeredPaneIDs := make(map[string]bool)
	var result ListAgentsResult
	panes, err := s.lister.ListPanes(ctx)
	if err != nil {
		logf(s.logger, "list panes: %v", err)
		result.Warning = "failed to inspect tmux panes; unregistered_panes may be incomplete"
	}

	for _, a := range agents {
		registeredPaneIDs[a.PaneID] = true
		if a.Name == orchestratorAgentName {
			result.Orchestrator = &OrchestratorEntry{PaneID: a.PaneID}
		} else {
			result.Agents = append(result.Agents, AgentEntry{
				Name:   a.Name,
				PaneID: a.PaneID,
				Role:   a.Role,
				Skills: a.Skills,
			})
		}
	}

	for _, p := range panes {
		if !registeredPaneIDs[p.ID] {
			result.Unregistered = append(result.Unregistered, p.ID)
		}
	}

	return result, nil
}
