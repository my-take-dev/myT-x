package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"myT-x/internal/orchestrator"
	"myT-x/internal/tmux"
)

func TestGetSessionEnlistmentContextAggregatesSavedTeamsAndRegisteredPanes(t *testing.T) {
	app := newOrchestratorTaskTestApp(t)
	db, tmpDir := createOrchestratorTaskTestDB(t, app)
	t.Cleanup(func() {
		_ = db.Close()
	})

	sessionSnapshot := createOrchestratorTestSession(t, app, "alpha", tmpDir)
	paneID := sessionSnapshot.Windows[0].Panes[0].ID

	if err := app.SaveOrchestratorTeam(OrchestratorTeamDefinition{
		ID:              "team-alpha",
		Name:            "Alpha",
		StorageLocation: orchestrator.StorageLocationGlobal,
		Members: []OrchestratorTeamMember{{
			ID:        "member-alpha",
			TeamID:    "team-alpha",
			Order:     0,
			PaneTitle: "Leader",
			Role:      "Lead engineer",
			Command:   "claude",
			Skills: []OrchestratorTeamMemberSkill{
				{Name: "Go", Description: "Backend services"},
				{Name: "React"},
			},
		}},
	}, "alpha"); err != nil {
		t.Fatalf("SaveOrchestratorTeam() error = %v", err)
	}

	if err := app.AddMemberToUnaffiliatedTeam(OrchestratorTeamMember{
		PaneTitle: "Ops",
		Role:      "Operations",
		Command:   "claude",
		Skills: []OrchestratorTeamMemberSkill{
			{Name: "React", Description: "UI delivery"},
		},
	}, orchestrator.StorageLocationGlobal, "alpha"); err != nil {
		t.Fatalf("AddMemberToUnaffiliatedTeam() error = %v", err)
	}

	if _, err := db.Exec(
		`INSERT INTO agents (name, pane_id, role, skills, mcp_instance_id, created_at) VALUES (?, ?, ?, ?, ?, datetime('now'))`,
		"leader", paneID, "Lead engineer", `[{"name":"Go"}]`, nil,
	); err != nil {
		t.Fatalf("insert registered agent: %v", err)
	}

	got, err := app.GetSessionEnlistmentContext("alpha")
	if err != nil {
		t.Fatalf("GetSessionEnlistmentContext() error = %v", err)
	}

	if len(got.Teams) != 2 {
		t.Fatalf("len(Teams) = %d, want 2", len(got.Teams))
	}
	if len(got.UnaffiliatedMembers) != 1 {
		t.Fatalf("len(UnaffiliatedMembers) = %d, want 1", len(got.UnaffiliatedMembers))
	}
	if got.RegisteredPaneIDs[0] != paneID {
		t.Fatalf("RegisteredPaneIDs = %#v, want [%s]", got.RegisteredPaneIDs, paneID)
	}
	if !strings.Contains(strings.Join(got.RoleCatalog, ","), "Lead engineer") || !strings.Contains(strings.Join(got.RoleCatalog, ","), "Operations") {
		t.Fatalf("RoleCatalog = %#v, want saved roles", got.RoleCatalog)
	}
	if len(got.SkillCatalog) != 2 {
		t.Fatalf("len(SkillCatalog) = %d, want 2", len(got.SkillCatalog))
	}
	reactSkill := findSkillByName(got.SkillCatalog, "React")
	if reactSkill == nil {
		t.Fatalf("SkillCatalog = %#v, want React entry", got.SkillCatalog)
	}
	if reactSkill.Description != "UI delivery" {
		t.Fatalf("React description = %q, want UI delivery", reactSkill.Description)
	}
}

func TestGetSessionEnlistmentContextTreatsMissingDBAsNoRegisteredPanes(t *testing.T) {
	app := newOrchestratorTaskTestApp(t)
	createOrchestratorTestSession(t, app, "alpha", t.TempDir())

	got, err := app.GetSessionEnlistmentContext("alpha")
	if err != nil {
		t.Fatalf("GetSessionEnlistmentContext() error = %v, want nil", err)
	}
	if len(got.RegisteredPaneIDs) != 0 {
		t.Fatalf("RegisteredPaneIDs = %#v, want empty", got.RegisteredPaneIDs)
	}
}

func TestEnlistPaneInitializesMissingDBBeforeSavingMember(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})
	runtimeEventsEmitFn = func(context.Context, string, ...any) {}

	app := newOrchestratorTaskTestApp(t)
	app.setRuntimeContext(context.Background())
	app.router = &tmux.CommandRouter{}

	projectRoot := t.TempDir()
	sessionSnapshot := createOrchestratorTestSession(t, app, "alpha", projectRoot)
	paneID := sessionSnapshot.Windows[0].Panes[0].ID

	mocks := withOrchestratorStartMocks(t, app)
	mocks.sendKeysPaste = func(string, string) error {
		return nil
	}

	if err := app.SaveOrchestratorTeam(OrchestratorTeamDefinition{
		ID:              "team-alpha",
		Name:            "Alpha",
		StorageLocation: orchestrator.StorageLocationGlobal,
		Members:         []OrchestratorTeamMember{},
	}, "alpha"); err != nil {
		t.Fatalf("SaveOrchestratorTeam() error = %v", err)
	}

	result, err := app.EnlistPane(EnlistPaneRequest{
		SessionName:     "alpha",
		PaneID:          paneID,
		TeamID:          "team-alpha",
		StorageLocation: orchestrator.StorageLocationGlobal,
		PaneState:       orchestrator.PaneStateCLIRunning,
		Member: OrchestratorTeamMember{
			PaneTitle: "Worker",
			Role:      "QA engineer",
			Command:   "claude",
		},
	})
	if err != nil {
		t.Fatalf("EnlistPane() error = %v", err)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("Warnings = %#v, want none", result.Warnings)
	}

	agents, err := app.ListOrchestratorAgents("alpha")
	if err != nil {
		t.Fatalf("ListOrchestratorAgents() error = %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("len(agents) = %d, want 1", len(agents))
	}
	if agents[0].Name != "worker" || agents[0].PaneID != paneID {
		t.Fatalf("agents = %#v, want worker on %s", agents, paneID)
	}

	legacyDBPath := filepath.Join(projectRoot, ".myT-x", "orchestrator.db")
	if _, statErr := os.Stat(legacyDBPath); !os.IsNotExist(statErr) {
		t.Fatalf("legacy db stat error = %v, want not exist", statErr)
	}
}

func TestEnlistPaneSavesMemberRegistersAgentAndBootstrapsPane(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	var emittedEventName string
	var emittedPayload any
	runtimeEventsEmitFn = func(_ context.Context, name string, args ...any) {
		emittedEventName = name
		if len(args) > 0 {
			emittedPayload = args[0]
		}
	}

	app := newOrchestratorTaskTestApp(t)
	app.setRuntimeContext(context.Background())
	app.router = &tmux.CommandRouter{}

	db, tmpDir := createOrchestratorTaskTestDB(t, app)
	t.Cleanup(func() {
		_ = db.Close()
	})
	sessionSnapshot := createOrchestratorTestSession(t, app, "alpha", tmpDir)
	paneID := sessionSnapshot.Windows[0].Panes[0].ID

	mocks := withOrchestratorStartMocks(t, app)
	var bootstrappedPaneID string
	var bootstrappedMessage string
	mocks.sendKeysPaste = func(paneID, text string) error {
		bootstrappedPaneID = paneID
		bootstrappedMessage = text
		return nil
	}

	if err := app.SaveOrchestratorTeam(OrchestratorTeamDefinition{
		ID:              "team-alpha",
		Name:            "Alpha",
		StorageLocation: orchestrator.StorageLocationGlobal,
		Members:         []OrchestratorTeamMember{},
	}, "alpha"); err != nil {
		t.Fatalf("SaveOrchestratorTeam() error = %v", err)
	}

	result, err := app.EnlistPane(EnlistPaneRequest{
		SessionName:     "alpha",
		PaneID:          paneID,
		TeamID:          "team-alpha",
		StorageLocation: orchestrator.StorageLocationGlobal,
		PaneState:       orchestrator.PaneStateCLIRunning,
		Member: OrchestratorTeamMember{
			PaneTitle: "Worker",
			Role:      "QA engineer",
			Command:   "claude",
			Skills: []OrchestratorTeamMemberSkill{
				{Name: "Testing", Description: "Regression coverage"},
			},
		},
	})
	if err != nil {
		t.Fatalf("EnlistPane() error = %v", err)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("Warnings = %#v, want none", result.Warnings)
	}

	teams, err := app.LoadOrchestratorTeams("alpha")
	if err != nil {
		t.Fatalf("LoadOrchestratorTeams() error = %v", err)
	}
	if len(teams) != 1 {
		t.Fatalf("len(teams) = %d, want 1", len(teams))
	}
	if len(teams[0].Members) != 1 {
		t.Fatalf("len(team members) = %d, want 1", len(teams[0].Members))
	}
	if teams[0].Members[0].PaneTitle != "Worker" {
		t.Fatalf("saved member pane title = %q, want Worker", teams[0].Members[0].PaneTitle)
	}

	agents, err := app.ListOrchestratorAgents("alpha")
	if err != nil {
		t.Fatalf("ListOrchestratorAgents() error = %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("len(agents) = %d, want 1", len(agents))
	}
	if agents[0].PaneID != paneID || agents[0].Role != "QA engineer" {
		t.Fatalf("agents = %#v, want pane %s / QA engineer", agents, paneID)
	}

	var rawSkills string
	if err := db.QueryRow(`SELECT skills FROM agents WHERE pane_id = ?`, paneID).Scan(&rawSkills); err != nil {
		t.Fatalf("query provisional agent skills: %v", err)
	}
	var skills []map[string]string
	if err := json.Unmarshal([]byte(rawSkills), &skills); err != nil {
		t.Fatalf("unmarshal skills: %v", err)
	}
	if len(skills) != 1 || skills[0]["name"] != "Testing" {
		t.Fatalf("stored skills = %#v, want Testing", skills)
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM agent_status WHERE agent_name = ?`, "worker").Scan(&status); err != nil {
		t.Fatalf("query provisional agent status: %v", err)
	}
	if status != "idle" {
		t.Fatalf("status = %q, want idle", status)
	}

	if emittedEventName != "orchestrator:agents-updated" {
		t.Fatalf("emitted event = %q, want orchestrator:agents-updated", emittedEventName)
	}
	payloadMap, ok := emittedPayload.(map[string]any)
	if !ok {
		t.Fatalf("emitted payload type = %T, want map[string]any", emittedPayload)
	}
	if payloadMap["sessionName"] != "alpha" {
		t.Fatalf("payload sessionName = %#v, want alpha", payloadMap["sessionName"])
	}
	if bootstrappedPaneID != paneID {
		t.Fatalf("bootstrapped pane = %q, want %s", bootstrappedPaneID, paneID)
	}
	if !strings.Contains(bootstrappedMessage, `register_agent(`) {
		t.Fatalf("bootstrap message = %q, want register_agent instructions", bootstrappedMessage)
	}
}

func TestEnlistPaneRejectsMissingPaneBeforeSavingMember(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	var emittedEventName string
	runtimeEventsEmitFn = func(_ context.Context, name string, args ...any) {
		emittedEventName = name
	}

	app := newOrchestratorTaskTestApp(t)
	app.setRuntimeContext(context.Background())
	app.router = &tmux.CommandRouter{}

	db, tmpDir := createOrchestratorTaskTestDB(t, app)
	t.Cleanup(func() {
		_ = db.Close()
	})
	createOrchestratorTestSession(t, app, "alpha", tmpDir)

	if err := app.SaveOrchestratorTeam(OrchestratorTeamDefinition{
		ID:              "team-alpha",
		Name:            "Alpha",
		StorageLocation: orchestrator.StorageLocationGlobal,
		Members:         []OrchestratorTeamMember{},
	}, "alpha"); err != nil {
		t.Fatalf("SaveOrchestratorTeam() error = %v", err)
	}
	if _, err := app.EnlistPane(EnlistPaneRequest{
		SessionName:     "alpha",
		PaneID:          "%999",
		TeamID:          "team-alpha",
		StorageLocation: orchestrator.StorageLocationGlobal,
		PaneState:       orchestrator.PaneStateCLIRunning,
		Member: OrchestratorTeamMember{
			PaneTitle: "Worker",
			Role:      "QA engineer",
			Command:   "claude",
		},
	}); err == nil || !strings.Contains(err.Error(), "pane %999 not found") {
		t.Fatalf("EnlistPane() error = %v, want missing pane error", err)
	}

	teams, err := app.LoadOrchestratorTeams("alpha")
	if err != nil {
		t.Fatalf("LoadOrchestratorTeams() error = %v", err)
	}
	if len(teams[0].Members) != 0 {
		t.Fatalf("len(team members) = %d, want 0", len(teams[0].Members))
	}

	var agentCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM agents`).Scan(&agentCount); err != nil {
		t.Fatalf("count agents: %v", err)
	}
	if agentCount != 0 {
		t.Fatalf("agent count = %d, want 0", agentCount)
	}
	if emittedEventName != "" {
		t.Fatalf("unexpected emitted event = %q", emittedEventName)
	}
}

func TestEnlistPaneRollsBackWhenProvisionalRegistrationFails(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	var emittedEventName string
	runtimeEventsEmitFn = func(_ context.Context, name string, args ...any) {
		emittedEventName = name
	}

	app := newOrchestratorTaskTestApp(t)
	app.setRuntimeContext(context.Background())
	app.router = &tmux.CommandRouter{}

	db, tmpDir := createOrchestratorTaskTestDB(t, app)
	t.Cleanup(func() {
		_ = db.Close()
	})
	sessionSnapshot := createOrchestratorTestSession(t, app, "alpha", tmpDir)
	paneID := sessionSnapshot.Windows[0].Panes[0].ID

	mocks := withOrchestratorStartMocks(t, app)
	var bootstrappedPaneID string
	mocks.sendKeysPaste = func(targetPaneID, text string) error {
		bootstrappedPaneID = targetPaneID
		return nil
	}

	if err := app.SaveOrchestratorTeam(OrchestratorTeamDefinition{
		ID:              "team-alpha",
		Name:            "Alpha",
		StorageLocation: orchestrator.StorageLocationGlobal,
		Members:         []OrchestratorTeamMember{},
	}, "alpha"); err != nil {
		t.Fatalf("SaveOrchestratorTeam() error = %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO agents (name, pane_id, role, skills, mcp_instance_id, created_at) VALUES (?, ?, ?, ?, ?, datetime('now'))`,
		"worker", "%9", "Existing worker", `[]`, "mcp-1",
	); err != nil {
		t.Fatalf("insert existing registered agent: %v", err)
	}

	if _, err := app.EnlistPane(EnlistPaneRequest{
		SessionName:     "alpha",
		PaneID:          paneID,
		TeamID:          "team-alpha",
		StorageLocation: orchestrator.StorageLocationGlobal,
		PaneState:       orchestrator.PaneStateCLIRunning,
		Member: OrchestratorTeamMember{
			PaneTitle: "Worker",
			Role:      "QA engineer",
			Command:   "claude",
		},
	}); err == nil || !strings.Contains(err.Error(), "provisional registration failed") {
		t.Fatalf("EnlistPane() error = %v, want provisional registration failure", err)
	}
	if bootstrappedPaneID != "" {
		t.Fatalf("bootstrap should be skipped, got pane %q", bootstrappedPaneID)
	}
	if emittedEventName != "" {
		t.Fatalf("unexpected emitted event = %q", emittedEventName)
	}

	var gotPaneID string
	var gotInstanceID *string
	if err := db.QueryRow(`SELECT pane_id, mcp_instance_id FROM agents WHERE name = ?`, "worker").Scan(&gotPaneID, &gotInstanceID); err != nil {
		t.Fatalf("query existing agent after enlist: %v", err)
	}
	if gotPaneID != "%9" {
		t.Fatalf("existing agent pane_id = %q, want %%9", gotPaneID)
	}
	if gotInstanceID == nil || *gotInstanceID != "mcp-1" {
		t.Fatalf("existing agent mcp_instance_id = %#v, want mcp-1", gotInstanceID)
	}

	teams, err := app.LoadOrchestratorTeams("alpha")
	if err != nil {
		t.Fatalf("LoadOrchestratorTeams() error = %v", err)
	}
	if len(teams[0].Members) != 0 {
		t.Fatalf("len(team members) = %d, want 0 after rollback", len(teams[0].Members))
	}
}

func TestEnlistPaneRollsBackWhenBootstrapFails(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	var emittedEventName string
	runtimeEventsEmitFn = func(_ context.Context, name string, args ...any) {
		emittedEventName = name
	}

	app := newOrchestratorTaskTestApp(t)
	app.setRuntimeContext(context.Background())
	app.router = &tmux.CommandRouter{}

	db, tmpDir := createOrchestratorTaskTestDB(t, app)
	t.Cleanup(func() {
		_ = db.Close()
	})
	sessionSnapshot := createOrchestratorTestSession(t, app, "alpha", tmpDir)
	paneID := sessionSnapshot.Windows[0].Panes[0].ID

	mocks := withOrchestratorStartMocks(t, app)
	mocks.sendKeysPaste = func(targetPaneID, text string) error {
		if strings.Contains(text, "register_agent(") {
			return context.Canceled
		}
		return nil
	}

	if err := app.SaveOrchestratorTeam(OrchestratorTeamDefinition{
		ID:              "team-alpha",
		Name:            "Alpha",
		StorageLocation: orchestrator.StorageLocationGlobal,
		Members:         []OrchestratorTeamMember{},
	}, "alpha"); err != nil {
		t.Fatalf("SaveOrchestratorTeam() error = %v", err)
	}

	if _, err := app.EnlistPane(EnlistPaneRequest{
		SessionName:     "alpha",
		PaneID:          paneID,
		TeamID:          "team-alpha",
		StorageLocation: orchestrator.StorageLocationGlobal,
		PaneState:       orchestrator.PaneStateCLIRunning,
		Member: OrchestratorTeamMember{
			PaneTitle: "Worker",
			Role:      "QA engineer",
			Command:   "claude",
		},
	}); err == nil || !strings.Contains(err.Error(), "bootstrap failed") {
		t.Fatalf("EnlistPane() error = %v, want bootstrap failure", err)
	}
	if emittedEventName != "" {
		t.Fatalf("unexpected emitted event = %q", emittedEventName)
	}

	teams, err := app.LoadOrchestratorTeams("alpha")
	if err != nil {
		t.Fatalf("LoadOrchestratorTeams() error = %v", err)
	}
	if len(teams[0].Members) != 0 {
		t.Fatalf("len(team members) = %d, want 0 after rollback", len(teams[0].Members))
	}

	var agentCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM agents WHERE pane_id = ?`, paneID).Scan(&agentCount); err != nil {
		t.Fatalf("count agents by pane: %v", err)
	}
	if agentCount != 0 {
		t.Fatalf("agent count = %d, want 0 after rollback", agentCount)
	}
}

func findSkillByName(skills []orchestrator.TeamMemberSkill, name string) *orchestrator.TeamMemberSkill {
	for i := range skills {
		if skills[i].Name == name {
			return &skills[i]
		}
	}
	return nil
}
