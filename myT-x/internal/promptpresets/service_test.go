package promptpresets

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"myT-x/internal/tmux"
)

func testDeps(t *testing.T, configPath string) Deps {
	t.Helper()

	return Deps{
		ConfigPath: func() string { return configPath },
		FindSessionSnapshot: func(sessionName string) (tmux.SessionSnapshot, error) {
			return tmux.SessionSnapshot{
				Name:     sessionName,
				RootPath: filepath.Join(filepath.Dir(configPath), "sessions", sessionName),
			}, nil
		},
	}
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	return NewService(testDeps(t, filepath.Join(t.TempDir(), "config.yaml")))
}

func TestNewServicePanicsOnMissingDeps(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for missing deps")
		}
	}()
	NewService(Deps{})
}

func TestPromptPresetFieldCountGuard(t *testing.T) {
	const want = 5
	if got := reflect.TypeFor[PromptPreset]().NumField(); got != want {
		t.Fatalf("PromptPreset field count = %d, want %d", got, want)
	}
}

func TestSaveValidation(t *testing.T) {
	s := newTestService(t)

	tests := []struct {
		name   string
		preset PromptPreset
		want   string
	}{
		{
			name:   "missing name",
			preset: PromptPreset{Body: "body"},
			want:   "name is required",
		},
		{
			name:   "missing body",
			preset: PromptPreset{Name: "name"},
			want:   "body is required",
		},
		{
			name:   "invalid storage location",
			preset: PromptPreset{Name: "name", Body: "body", StorageLocation: "workspace"},
			want:   "unsupported prompt preset storage location",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.Save(tt.preset, "")
			if err == nil {
				t.Fatal("Save() expected error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Save() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestSaveAndLoadGlobalAndProjectPresets(t *testing.T) {
	s := newTestService(t)

	if err := s.Save(PromptPreset{
		Name: "Global preset",
		Body: "Explain the change only.",
	}, ""); err != nil {
		t.Fatalf("Save(global) error = %v", err)
	}

	if err := s.Save(PromptPreset{
		Name:            "Project preset",
		Body:            "Run tests first.",
		StorageLocation: StorageLocationProject,
	}, "alpha"); err != nil {
		t.Fatalf("Save(project) error = %v", err)
	}

	globalResult, err := s.Load("")
	if err != nil {
		t.Fatalf("Load(global) error = %v", err)
	}
	globalPresets := globalResult.Presets
	if len(globalPresets) != 1 {
		t.Fatalf("len(globalPresets) = %d, want 1", len(globalPresets))
	}
	if len(globalResult.Warnings) != 0 {
		t.Fatalf("Load(global) warnings = %#v, want none", globalResult.Warnings)
	}
	if globalPresets[0].StorageLocation != StorageLocationGlobal {
		t.Fatalf("globalPresets[0].StorageLocation = %q, want %q", globalPresets[0].StorageLocation, StorageLocationGlobal)
	}

	mergedResult, err := s.Load("alpha")
	if err != nil {
		t.Fatalf("Load(project) error = %v", err)
	}
	mergedPresets := mergedResult.Presets
	if len(mergedPresets) != 2 {
		t.Fatalf("len(mergedPresets) = %d, want 2", len(mergedPresets))
	}
	if len(mergedResult.Warnings) != 0 {
		t.Fatalf("Load(project) warnings = %#v, want none", mergedResult.Warnings)
	}
	if mergedPresets[0].Name != "Global preset" || mergedPresets[0].StorageLocation != StorageLocationGlobal {
		t.Fatalf("mergedPresets[0] = %#v, want global preset", mergedPresets[0])
	}
	if mergedPresets[1].Name != "Project preset" || mergedPresets[1].StorageLocation != StorageLocationProject {
		t.Fatalf("mergedPresets[1] = %#v, want project preset", mergedPresets[1])
	}
}

func TestSaveUpdatesExistingPresetWithoutChangingOrder(t *testing.T) {
	s := newTestService(t)

	if err := s.Save(PromptPreset{Name: "First", Body: "one"}, ""); err != nil {
		t.Fatalf("Save(first) error = %v", err)
	}
	if err := s.Save(PromptPreset{Name: "Second", Body: "two"}, ""); err != nil {
		t.Fatalf("Save(second) error = %v", err)
	}

	loadResult, err := s.Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	presets := loadResult.Presets
	updated := presets[0]
	updated.Name = "First updated"
	updated.Body = "one updated"

	if err := s.Save(updated, ""); err != nil {
		t.Fatalf("Save(update) error = %v", err)
	}

	reloadedResult, err := s.Load("")
	if err != nil {
		t.Fatalf("Load() after update error = %v", err)
	}
	reloaded := reloadedResult.Presets
	if len(reloaded) != 2 {
		t.Fatalf("len(reloaded) = %d, want 2", len(reloaded))
	}
	if reloaded[0].Name != "First updated" {
		t.Fatalf("reloaded[0].Name = %q, want %q", reloaded[0].Name, "First updated")
	}
	if reloaded[0].Order != 0 || reloaded[1].Order != 1 {
		t.Fatalf("orders = [%d, %d], want [0, 1]", reloaded[0].Order, reloaded[1].Order)
	}
}

func TestSaveRejectsLimitExceeded(t *testing.T) {
	s := newTestService(t)

	for index := range MaxPresets {
		if err := s.Save(PromptPreset{
			Name: "Preset " + strconv.Itoa(index),
			Body: "body",
		}, ""); err != nil {
			t.Fatalf("Save(%d) error = %v", index, err)
		}
	}

	err := s.Save(PromptPreset{Name: "Overflow", Body: "body"}, "")
	if err == nil {
		t.Fatal("Save() expected limit error")
	}
	if !strings.Contains(err.Error(), "must be 200 or fewer") {
		t.Fatalf("Save() error = %v, want limit error", err)
	}
}

func TestDeletePreset(t *testing.T) {
	s := newTestService(t)

	if err := s.Save(PromptPreset{Name: "First", Body: "one"}, ""); err != nil {
		t.Fatalf("Save(first) error = %v", err)
	}
	if err := s.Save(PromptPreset{Name: "Second", Body: "two"}, ""); err != nil {
		t.Fatalf("Save(second) error = %v", err)
	}

	loadResult, err := s.Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	presets := loadResult.Presets

	if err := s.Delete(presets[0].ID, StorageLocationGlobal, ""); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if err := s.Delete("missing", StorageLocationGlobal, ""); err != nil {
		t.Fatalf("Delete(missing) error = %v", err)
	}

	reloadedResult, err := s.Load("")
	if err != nil {
		t.Fatalf("Load() after delete error = %v", err)
	}
	reloaded := reloadedResult.Presets
	if len(reloaded) != 1 {
		t.Fatalf("len(reloaded) = %d, want 1", len(reloaded))
	}
	if reloaded[0].Name != "Second" {
		t.Fatalf("reloaded[0].Name = %q, want %q", reloaded[0].Name, "Second")
	}
	if reloaded[0].Order != 0 {
		t.Fatalf("reloaded[0].Order = %d, want 0", reloaded[0].Order)
	}
}

func TestReorderPresets(t *testing.T) {
	s := newTestService(t)

	for _, preset := range []PromptPreset{
		{Name: "Alpha", Body: "one"},
		{Name: "Beta", Body: "two"},
		{Name: "Gamma", Body: "three"},
	} {
		if err := s.Save(preset, ""); err != nil {
			t.Fatalf("Save(%q) error = %v", preset.Name, err)
		}
	}

	loadResult, err := s.Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	presets := loadResult.Presets

	if err := s.Reorder([]string{presets[2].ID, presets[0].ID}, StorageLocationGlobal, ""); err != nil {
		t.Fatalf("Reorder() error = %v", err)
	}

	reloadedResult, err := s.Load("")
	if err != nil {
		t.Fatalf("Load() after reorder error = %v", err)
	}
	reloaded := reloadedResult.Presets
	gotNames := []string{reloaded[0].Name, reloaded[1].Name, reloaded[2].Name}
	wantNames := []string{"Gamma", "Alpha", "Beta"}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("names after reorder = %#v, want %#v", gotNames, wantNames)
	}

	if err := s.Reorder(nil, StorageLocationGlobal, ""); err != nil {
		t.Fatalf("Reorder(nil) error = %v", err)
	}
	if err := s.Reorder([]string{"missing"}, StorageLocationGlobal, ""); err == nil {
		t.Fatal("Reorder(missing) expected error")
	}
	if err := s.Reorder([]string{reloaded[0].ID, reloaded[0].ID}, StorageLocationGlobal, ""); err == nil {
		t.Fatal("Reorder(duplicate) expected error")
	}
}

func TestLoadReturnsEmptyWhenProjectFileMissing(t *testing.T) {
	s := newTestService(t)

	loadResult, err := s.Load("missing-project")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	presets := loadResult.Presets
	if len(presets) != 0 {
		t.Fatalf("len(presets) = %d, want 0", len(presets))
	}
	if len(loadResult.Warnings) != 0 {
		t.Fatalf("Load() warnings = %#v, want none", loadResult.Warnings)
	}
}

func TestLoadReturnsGlobalPresetsWhenProjectRootUnavailable(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	s := NewService(Deps{
		ConfigPath: func() string { return configPath },
		FindSessionSnapshot: func(sessionName string) (tmux.SessionSnapshot, error) {
			return tmux.SessionSnapshot{Name: sessionName}, nil
		},
	})

	if err := s.Save(PromptPreset{Name: "Global preset", Body: "Global body"}, ""); err != nil {
		t.Fatalf("Save(global) error = %v", err)
	}

	loadResult, err := s.Load("alpha")
	if err != nil {
		t.Fatalf("Load(alpha) error = %v", err)
	}
	presets := loadResult.Presets
	if len(presets) != 1 {
		t.Fatalf("len(presets) = %d, want 1", len(presets))
	}
	if len(loadResult.Warnings) != 1 {
		t.Fatalf("Load(alpha) warnings = %#v, want 1 warning", loadResult.Warnings)
	}
	if !strings.Contains(loadResult.Warnings[0], "alpha") {
		t.Fatalf("Load(alpha) warning = %q, want session name", loadResult.Warnings[0])
	}
	if presets[0].Name != "Global preset" {
		t.Fatalf("presets[0].Name = %q, want %q", presets[0].Name, "Global preset")
	}
	if presets[0].StorageLocation != StorageLocationGlobal {
		t.Fatalf("presets[0].StorageLocation = %q, want %q", presets[0].StorageLocation, StorageLocationGlobal)
	}
}

func TestMalformedFileReadModeAndWriteMode(t *testing.T) {
	s := newTestService(t)
	path := s.resolveGlobalStoragePath()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte("{invalid"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	loadResult, err := s.Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	presets := loadResult.Presets
	if len(presets) != 0 {
		t.Fatalf("len(presets) = %d, want 0", len(presets))
	}
	if len(loadResult.Warnings) != 1 {
		t.Fatalf("Load() warnings = %#v, want malformed warning", loadResult.Warnings)
	}

	err = s.Save(PromptPreset{Name: "Preset", Body: "body"}, "")
	if err == nil {
		t.Fatal("Save() expected malformed JSON error")
	}
	if !strings.Contains(err.Error(), "parse prompt presets") {
		t.Fatalf("Save() error = %v, want parse error", err)
	}

	if _, statErr := os.Stat(path + ".bak"); statErr != nil {
		t.Fatalf("backup file missing: %v", statErr)
	}
}

func TestSaveAndLoadPreservePromptBodyWhitespace(t *testing.T) {
	s := newTestService(t)
	body := "\nKeep the leading newline.\nAnd the trailing newline.\n"

	if err := s.Save(PromptPreset{Name: "Whitespace", Body: body}, ""); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loadResult, err := s.Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loadResult.Presets) != 1 {
		t.Fatalf("len(Load().Presets) = %d, want 1", len(loadResult.Presets))
	}
	if loadResult.Presets[0].Body != body {
		t.Fatalf("Body = %q, want %q", loadResult.Presets[0].Body, body)
	}
}
