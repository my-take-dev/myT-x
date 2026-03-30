package main

// BootstrapMemberToPane bootstraps a single member to an existing pane.
// Wails-bound: called from the frontend addTermMember flow.
func (a *App) BootstrapMemberToPane(request BootstrapMemberToPaneRequest) (BootstrapMemberToPaneResult, error) {
	return a.orchestratorService.BootstrapMemberToPane(request)
}

// AddMemberToUnaffiliatedTeam adds a member to the unaffiliated (system) team.
// Wails-bound: called from the frontend addTermMember flow.
func (a *App) AddMemberToUnaffiliatedTeam(member OrchestratorTeamMember, storageLocation string, sessionName string) error {
	return a.orchestratorService.AddMemberToUnaffiliatedTeam(member, storageLocation, sessionName)
}

// EnsureUnaffiliatedTeam returns the unaffiliated team, creating it if needed.
// Wails-bound: called from the frontend addTermMember flow.
func (a *App) EnsureUnaffiliatedTeam(storageLocation string, sessionName string) (OrchestratorTeamDefinition, error) {
	return a.orchestratorService.EnsureUnaffiliatedTeam(storageLocation, sessionName)
}

// SaveUnaffiliatedTeamMembers replaces all members of the unaffiliated team.
// Wails-bound: called from the frontend TeamEditor when editing the system team.
func (a *App) SaveUnaffiliatedTeamMembers(members []OrchestratorTeamMember, sessionName string) error {
	return a.orchestratorService.SaveUnaffiliatedTeamMembers(members, sessionName)
}
