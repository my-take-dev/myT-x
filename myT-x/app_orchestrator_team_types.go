package main

import (
	"myT-x/internal/orchestrator"
)

// Type aliases for Wails binding compatibility.
// Wails generates bindings from App method signatures in the main package.
// These aliases re-export internal orchestrator types so that Wails can
// discover them without exposing the internal package directly.
type OrchestratorTeamDefinition = orchestrator.TeamDefinition
type OrchestratorTeamMember = orchestrator.TeamMember
type OrchestratorTeamMemberSkill = orchestrator.TeamMemberSkill
type StartOrchestratorTeamRequest = orchestrator.StartTeamRequest
type StartOrchestratorTeamResult = orchestrator.StartTeamResult
type BootstrapMemberToPaneRequest = orchestrator.BootstrapMemberToPaneRequest
type BootstrapMemberToPaneResult = orchestrator.BootstrapMemberToPaneResult
