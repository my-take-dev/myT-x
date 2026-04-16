package main

import (
	"path/filepath"
	"reflect"
	"testing"

	"myT-x/internal/config"
	"myT-x/internal/promptpresets"
	"myT-x/internal/tmux"
)

func newPromptPresetTestApp(t *testing.T) *App {
	t.Helper()

	app := NewApp()
	app.configState.Initialize(filepath.Join(t.TempDir(), "config.yaml"), config.DefaultConfig())
	app.sessions = tmux.NewSessionManager()
	return app
}

func createPromptPresetSourceSession(t *testing.T, app *App, sessionName, rootPath string) {
	t.Helper()

	if _, _, err := app.sessions.CreateSession(sessionName, "main", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if err := app.sessions.SetRootPath(sessionName, rootPath); err != nil {
		t.Fatalf("SetRootPath() error = %v", err)
	}
}

func TestSaveAndLoadPromptPresets(t *testing.T) {
	app := newPromptPresetTestApp(t)

	if err := app.SavePromptPreset(PromptPreset{
		Name: "Global preset",
		Body: "Summarize only.",
	}, ""); err != nil {
		t.Fatalf("SavePromptPreset(global) error = %v", err)
	}

	loadResult, err := app.LoadPromptPresets("")
	if err != nil {
		t.Fatalf("LoadPromptPresets() error = %v", err)
	}
	presets := loadResult.Presets
	if len(presets) != 1 {
		t.Fatalf("len(presets) = %d, want 1", len(presets))
	}
	if presets[0].StorageLocation != promptpresets.StorageLocationGlobal {
		t.Fatalf("presets[0].StorageLocation = %q, want %q", presets[0].StorageLocation, promptpresets.StorageLocationGlobal)
	}
}

func TestLoadPromptPresetsMergesProjectPresets(t *testing.T) {
	app := newPromptPresetTestApp(t)
	projectRoot := filepath.Join(t.TempDir(), "workspace")
	createPromptPresetSourceSession(t, app, "alpha", projectRoot)

	if err := app.SavePromptPreset(PromptPreset{
		Name: "Global preset",
		Body: "Global body",
	}, ""); err != nil {
		t.Fatalf("SavePromptPreset(global) error = %v", err)
	}
	if err := app.SavePromptPreset(PromptPreset{
		Name:            "Project preset",
		Body:            "Project body",
		StorageLocation: promptpresets.StorageLocationProject,
	}, "alpha"); err != nil {
		t.Fatalf("SavePromptPreset(project) error = %v", err)
	}

	loadResult, err := app.LoadPromptPresets("alpha")
	if err != nil {
		t.Fatalf("LoadPromptPresets(project) error = %v", err)
	}
	presets := loadResult.Presets
	if len(presets) != 2 {
		t.Fatalf("len(presets) = %d, want 2", len(presets))
	}
	if presets[0].StorageLocation != promptpresets.StorageLocationGlobal {
		t.Fatalf("presets[0].StorageLocation = %q, want %q", presets[0].StorageLocation, promptpresets.StorageLocationGlobal)
	}
	if presets[1].StorageLocation != promptpresets.StorageLocationProject {
		t.Fatalf("presets[1].StorageLocation = %q, want %q", presets[1].StorageLocation, promptpresets.StorageLocationProject)
	}
}

func TestDeleteAndReorderPromptPresets(t *testing.T) {
	app := newPromptPresetTestApp(t)

	for _, preset := range []PromptPreset{
		{Name: "Alpha", Body: "one"},
		{Name: "Beta", Body: "two"},
		{Name: "Gamma", Body: "three"},
	} {
		if err := app.SavePromptPreset(preset, ""); err != nil {
			t.Fatalf("SavePromptPreset(%q) error = %v", preset.Name, err)
		}
	}

	loadResult, err := app.LoadPromptPresets("")
	if err != nil {
		t.Fatalf("LoadPromptPresets() error = %v", err)
	}
	presets := loadResult.Presets
	if err := app.ReorderPromptPresets([]string{presets[2].ID, presets[0].ID}, promptpresets.StorageLocationGlobal, ""); err != nil {
		t.Fatalf("ReorderPromptPresets() error = %v", err)
	}

	reorderedResult, err := app.LoadPromptPresets("")
	if err != nil {
		t.Fatalf("LoadPromptPresets() after reorder error = %v", err)
	}
	reordered := reorderedResult.Presets
	gotNames := []string{reordered[0].Name, reordered[1].Name, reordered[2].Name}
	wantNames := []string{"Gamma", "Alpha", "Beta"}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("names after reorder = %#v, want %#v", gotNames, wantNames)
	}

	if err := app.DeletePromptPreset(reordered[1].ID, promptpresets.StorageLocationGlobal, ""); err != nil {
		t.Fatalf("DeletePromptPreset() error = %v", err)
	}

	remainingResult, err := app.LoadPromptPresets("")
	if err != nil {
		t.Fatalf("LoadPromptPresets() after delete error = %v", err)
	}
	remaining := remainingResult.Presets
	if len(remaining) != 2 {
		t.Fatalf("len(remaining) = %d, want 2", len(remaining))
	}
	if remaining[0].Name != "Gamma" || remaining[1].Name != "Beta" {
		t.Fatalf("remaining names = %#v, want [Gamma Beta]", []string{remaining[0].Name, remaining[1].Name})
	}
}

func TestLoadPromptPresetsReturnsGlobalPresetsWhenSessionHasNoSourceRoot(t *testing.T) {
	app := newPromptPresetTestApp(t)

	if _, _, err := app.sessions.CreateSession("alpha", "main", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if err := app.SavePromptPreset(PromptPreset{
		Name: "Global preset",
		Body: "Global body",
	}, ""); err != nil {
		t.Fatalf("SavePromptPreset(global) error = %v", err)
	}

	loadResult, err := app.LoadPromptPresets("alpha")
	if err != nil {
		t.Fatalf("LoadPromptPresets(alpha) error = %v", err)
	}
	presets := loadResult.Presets
	if len(presets) != 1 {
		t.Fatalf("len(presets) = %d, want 1", len(presets))
	}
	if presets[0].StorageLocation != promptpresets.StorageLocationGlobal {
		t.Fatalf("presets[0].StorageLocation = %q, want %q", presets[0].StorageLocation, promptpresets.StorageLocationGlobal)
	}
}
