package main

// SaveOrchestratorTeam saves or updates a team definition.
// Wails-bound: called from the frontend orchestrator teams panel.
func (a *App) SaveOrchestratorTeam(team OrchestratorTeamDefinition, sessionName string) error {
	return a.orchestratorService.SaveTeam(team, sessionName)
}

// LoadOrchestratorTeams loads global and project-local team definitions.
// Wails-bound: called from the frontend orchestrator teams panel.
func (a *App) LoadOrchestratorTeams(sessionName string) ([]OrchestratorTeamDefinition, error) {
	return a.orchestratorService.LoadTeams(sessionName)
}

// DeleteOrchestratorTeam deletes a team definition.
// Wails-bound: called from the frontend orchestrator teams panel.
func (a *App) DeleteOrchestratorTeam(teamID string, storageLocation string, sessionName string) error {
	return a.orchestratorService.DeleteTeam(teamID, storageLocation, sessionName)
}

// ReorderOrchestratorTeams reorders team definitions.
// Wails-bound: called from the frontend orchestrator teams panel.
func (a *App) ReorderOrchestratorTeams(teamIDs []string, storageLocation string, sessionName string) error {
	return a.orchestratorService.ReorderTeams(teamIDs, storageLocation, sessionName)
}
