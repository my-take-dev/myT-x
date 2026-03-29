package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"myT-x/internal/orchestrator"
)

func TestSeedOrchestratorTeamSamplesViaLoadTeams(t *testing.T) {
	app := newOrchestratorTeamTestApp(t)
	definitionsPath, membersPath := app.resolveOrchestratorTeamStoragePaths()

	// Verify files do not exist yet.
	if _, err := os.Stat(definitionsPath); !os.IsNotExist(err) {
		t.Fatalf("definitions file should not exist before load, err = %v", err)
	}

	// Loading triggers seed on first call.
	teams, err := app.LoadOrchestratorTeams("")
	if err != nil {
		t.Fatalf("LoadOrchestratorTeams() error = %v", err)
	}
	if len(teams) == 0 {
		t.Fatal("expected seeded teams, got 0")
	}

	// Verify files were created.
	if _, err := os.Stat(definitionsPath); err != nil {
		t.Fatalf("definitions file not created: %v", err)
	}
	if _, err := os.Stat(membersPath); err != nil {
		t.Fatalf("members file not created: %v", err)
	}

	// Verify content is valid JSON.
	defsData, err := os.ReadFile(definitionsPath)
	if err != nil {
		t.Fatalf("read definitions: %v", err)
	}
	var defs []json.RawMessage
	if err := json.Unmarshal(defsData, &defs); err != nil {
		t.Fatalf("definitions JSON is invalid: %v", err)
	}
	if len(defs) == 0 {
		t.Fatal("seeded definitions should not be empty")
	}
}

func TestSeedOrchestratorTeamSkipsWhenFileExists(t *testing.T) {
	app := newOrchestratorTeamTestApp(t)
	definitionsPath, _ := app.resolveOrchestratorTeamStoragePaths()

	// Create definitions file with empty array (user deleted all teams).
	if err := os.MkdirAll(filepath.Dir(definitionsPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(definitionsPath, []byte("[]"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	teams, err := app.LoadOrchestratorTeams("")
	if err != nil {
		t.Fatalf("LoadOrchestratorTeams() error = %v", err)
	}
	if len(teams) != 0 {
		t.Fatalf("expected 0 teams (no re-seed), got %d", len(teams))
	}
}

func TestLoadOrchestratorTeamsProjectStorageMerge(t *testing.T) {
	app := newOrchestratorTeamTestApp(t)

	// Save a global team.
	if err := app.SaveOrchestratorTeam(OrchestratorTeamDefinition{
		ID:   "global-team",
		Name: "Global Team",
		Members: []OrchestratorTeamMember{
			{PaneTitle: "GlobalMember", Role: "Lead", Command: "codex"},
		},
	}, ""); err != nil {
		t.Fatalf("SaveOrchestratorTeam(global) error = %v", err)
	}

	// Load without session should return global teams only.
	teams, err := app.LoadOrchestratorTeams("")
	if err != nil {
		t.Fatalf("LoadOrchestratorTeams('') error = %v", err)
	}
	if len(teams) != 1 {
		t.Fatalf("len(teams) = %d, want 1", len(teams))
	}
	if teams[0].StorageLocation != orchestrator.StorageLocationGlobal {
		t.Fatalf("teams[0].StorageLocation = %q, want %q", teams[0].StorageLocation, orchestrator.StorageLocationGlobal)
	}
}

func TestReadOrchestratorTeamDefinitionsWithModeAllowMalformedReturnsEmpty(t *testing.T) {
	app := newOrchestratorTeamTestApp(t)
	definitionsPath, membersPath := app.resolveOrchestratorTeamStoragePaths()

	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(definitionsPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	// Write malformed JSON to both files.
	if err := os.WriteFile(definitionsPath, []byte("{invalid"), 0o644); err != nil {
		t.Fatalf("WriteFile(definitions) error = %v", err)
	}
	if err := os.WriteFile(membersPath, []byte("{invalid"), 0o644); err != nil {
		t.Fatalf("WriteFile(members) error = %v", err)
	}

	// Load (read mode) should return empty slice without error.
	teams, err := app.LoadOrchestratorTeams("")
	if err != nil {
		t.Fatalf("LoadOrchestratorTeams() with malformed JSON error = %v", err)
	}
	if len(teams) != 0 {
		t.Fatalf("expected empty teams, got %d", len(teams))
	}

	// Save (write mode) should refuse to overwrite malformed JSON.
	err = app.SaveOrchestratorTeam(OrchestratorTeamDefinition{
		Name: "Ops",
		Members: []OrchestratorTeamMember{{
			PaneTitle: "Ops",
			Role:      "Operations",
			Command:   "codex",
		}},
	}, "")
	if err == nil {
		t.Fatal("SaveOrchestratorTeam() expected malformed JSON error")
	}
	if !strings.Contains(err.Error(), "parse team definitions") {
		t.Fatalf("SaveOrchestratorTeam() error = %v, want 'parse team definitions'", err)
	}
}
