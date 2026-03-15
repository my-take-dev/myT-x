package sample_teams

import "embed"

//go:embed orchestrator-team-definitions.json
//go:embed orchestrator-team-members.json
var FS embed.FS
