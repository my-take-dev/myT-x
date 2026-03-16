package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSeedOrchestratorTeamSamplesInternalWritesWhenFileNotExists(t *testing.T) {
	dir := t.TempDir()
	definitionsPath := filepath.Join(dir, orchestratorTeamDefinitionsFileName)
	membersPath := filepath.Join(dir, orchestratorTeamMembersFileName)

	seeded := seedOrchestratorTeamSamplesInternal(definitionsPath, membersPath)
	if !seeded {
		t.Fatal("seedOrchestratorTeamSamplesInternal() = false, want true for first-time seeding")
	}

	// Verify files were created
	if _, err := os.Stat(definitionsPath); err != nil {
		t.Fatalf("definitions file not created: %v", err)
	}
	if _, err := os.Stat(membersPath); err != nil {
		t.Fatalf("members file not created: %v", err)
	}

	// Verify content is valid JSON
	defsData, err := os.ReadFile(definitionsPath)
	if err != nil {
		t.Fatalf("read definitions: %v", err)
	}
	var defs []orchestratorTeamFileRecord
	if err := json.Unmarshal(defsData, &defs); err != nil {
		t.Fatalf("definitions JSON is invalid: %v", err)
	}
	if len(defs) == 0 {
		t.Fatal("seeded definitions should not be empty")
	}
}

func TestSeedOrchestratorTeamSamplesInternalSkipsWhenFileExists(t *testing.T) {
	dir := t.TempDir()
	definitionsPath := filepath.Join(dir, orchestratorTeamDefinitionsFileName)
	membersPath := filepath.Join(dir, orchestratorTeamMembersFileName)

	// Create definitions file with empty array (user deleted all teams)
	if err := os.WriteFile(definitionsPath, []byte("[]"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	seeded := seedOrchestratorTeamSamplesInternal(definitionsPath, membersPath)
	if seeded {
		t.Fatal("seedOrchestratorTeamSamplesInternal() = true, want false when file exists")
	}
}

func TestWriteOrchestratorTeamJSONCreatesFileAndCleansTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "teams.json")

	payload := []orchestratorTeamFileRecord{
		{ID: "t1", Name: "Team1", Order: 0},
	}

	if err := writeOrchestratorTeamJSON(path, payload, "test"); err != nil {
		t.Fatalf("writeOrchestratorTeamJSON() error = %v", err)
	}

	// Verify file was created with correct content
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	var read []orchestratorTeamFileRecord
	if err := json.Unmarshal(data, &read); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(read) != 1 || read[0].ID != "t1" {
		t.Fatalf("got %+v, want [{ID:t1 ...}]", read)
	}

	// Verify no temp files left behind
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatalf("temp file not cleaned up: %s", entry.Name())
		}
	}
}

func TestRenameWithRetrySucceedsOnFirstAttempt(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")

	if err := os.WriteFile(src, []byte("data"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := renameWithRetry(src, dst); err != nil {
		t.Fatalf("renameWithRetry() error = %v", err)
	}

	// src should no longer exist
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatal("source file should not exist after rename")
	}

	// dst should contain original data
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(data) != "data" {
		t.Fatalf("dst content = %q, want %q", string(data), "data")
	}
}

func TestLoadOrchestratorTeamsProjectStorageMerge(t *testing.T) {
	app := newOrchestratorTeamTestApp(t)

	// Save a global team
	if err := app.SaveOrchestratorTeam(OrchestratorTeamDefinition{
		ID:   "global-team",
		Name: "Global Team",
		Members: []OrchestratorTeamMember{
			{PaneTitle: "GlobalMember", Role: "Lead", Command: "codex"},
		},
	}, ""); err != nil {
		t.Fatalf("SaveOrchestratorTeam(global) error = %v", err)
	}

	// Load without session should return global teams only
	teams, err := app.LoadOrchestratorTeams("")
	if err != nil {
		t.Fatalf("LoadOrchestratorTeams('') error = %v", err)
	}
	if len(teams) != 1 {
		t.Fatalf("len(teams) = %d, want 1", len(teams))
	}
	if teams[0].StorageLocation != orchestratorStorageLocationGlobal {
		t.Fatalf("teams[0].StorageLocation = %q, want %q", teams[0].StorageLocation, orchestratorStorageLocationGlobal)
	}
}

func TestReadOrchestratorTeamDefinitionsWithModeAllowMalformedReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "definitions.json")

	// Write malformed JSON
	if err := os.WriteFile(path, []byte("{invalid"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// allowMalformed=true should return empty slice without error
	defs, err := readOrchestratorTeamDefinitionsWithMode(path, true)
	if err != nil {
		t.Fatalf("readOrchestratorTeamDefinitionsWithMode(allowMalformed=true) error = %v", err)
	}
	if len(defs) != 0 {
		t.Fatalf("expected empty definitions, got %d", len(defs))
	}

	// allowMalformed=false should return error
	_, err = readOrchestratorTeamDefinitionsWithMode(path, false)
	if err == nil {
		t.Fatal("readOrchestratorTeamDefinitionsWithMode(allowMalformed=false) expected error")
	}
	if !strings.Contains(err.Error(), "parse team definitions") {
		t.Fatalf("error = %v, want 'parse team definitions'", err)
	}
}

func TestReadOrchestratorTeamDefinitionsWithModeFileNotExists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")

	defs, err := readOrchestratorTeamDefinitionsWithMode(path, true)
	if err != nil {
		t.Fatalf("readOrchestratorTeamDefinitionsWithMode(nonexistent) error = %v", err)
	}
	if len(defs) != 0 {
		t.Fatalf("expected empty definitions for nonexistent file, got %d", len(defs))
	}
}
