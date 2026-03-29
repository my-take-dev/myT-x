package main

// StartOrchestratorTeam launches a team of agents.
// Wails-bound: called from the frontend orchestrator teams panel.
func (a *App) StartOrchestratorTeam(request StartOrchestratorTeamRequest) (StartOrchestratorTeamResult, error) {
	return a.orchestratorService.StartTeam(request)
}
