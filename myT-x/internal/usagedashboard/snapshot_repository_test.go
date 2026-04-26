package usagedashboard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sampleSnapshot(workDir string, savedAt time.Time) PersistedSnapshot {
	return PersistedSnapshot{
		SchemaVersion: snapshotSchemaVersion,
		WorkDir:       workDir,
		SavedAt:       savedAt,
		Claude: &ClaudeUsageStats{
			TotalSessions:      3,
			Skills:             []UsageEntry{{Name: "go-test-patterns", Count: 1}},
			Agents:             []UsageEntry{},
			SlashCommands:      []UsageEntry{},
			SkillsDaily:        []DailyUsageSeries{},
			AgentsDaily:        []DailyUsageSeries{},
			SlashCommandsDaily: []DailyUsageSeries{},
			DailyActivity:      []DailyBucket{},
			Health:             SourceHealth{PartialErrors: []string{}},
		},
		Codex: &CodexUsageStats{
			TotalPrompts:  5,
			Skills:        []UsageEntry{},
			Agents:        []UsageEntry{{Name: "test-agent", Count: 2}},
			SkillsDaily:   []DailyUsageSeries{},
			AgentsDaily:   []DailyUsageSeries{},
			DailyActivity: []DailyBucket{},
			Health:        SourceHealth{PartialErrors: []string{}},
		},
	}
}

func TestFileSnapshotRepository_LoadNotExist(t *testing.T) {
	repo := NewFileSnapshotRepository()
	workDir := t.TempDir()

	snap, found, err := repo.Load(workDir)
	if err != nil {
		t.Fatalf("expected nil err for missing file, got: %v", err)
	}
	if found {
		t.Errorf("expected found=false for missing file, got snap=%+v", snap)
	}
}

func TestFileSnapshotRepository_SaveLoadRoundTrip(t *testing.T) {
	repo := NewFileSnapshotRepository()
	workDir := t.TempDir()
	saved := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	original := sampleSnapshot(workDir, saved)

	if err := repo.Save(original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// File must exist at the documented path.
	wantPath := filepath.Join(workDir, ".myT-x", "usage-dashboard.json")
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("expected file at %s: %v", wantPath, err)
	}

	loaded, found, err := repo.Load(workDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !found {
		t.Fatal("expected found=true after Save")
	}
	if loaded.SchemaVersion != snapshotSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", loaded.SchemaVersion, snapshotSchemaVersion)
	}
	if loaded.WorkDir != workDir {
		t.Errorf("WorkDir = %q, want %q", loaded.WorkDir, workDir)
	}
	if !loaded.SavedAt.Equal(saved) {
		t.Errorf("SavedAt = %v, want %v", loaded.SavedAt, saved)
	}
	if loaded.Claude == nil || loaded.Claude.TotalSessions != 3 {
		t.Errorf("Claude.TotalSessions = %v, want 3", loaded.Claude)
	}
	if loaded.Codex == nil || loaded.Codex.TotalPrompts != 5 {
		t.Errorf("Codex.TotalPrompts = %v, want 5", loaded.Codex)
	}
}

func TestFileSnapshotRepository_LoadCorrupted(t *testing.T) {
	repo := NewFileSnapshotRepository()
	workDir := t.TempDir()
	dir := filepath.Join(workDir, ".myT-x")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, "usage-dashboard.json")
	if err := os.WriteFile(path, []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}

	snap, found, err := repo.Load(workDir)
	if err == nil {
		t.Errorf("expected parse error for corrupt JSON, got snap=%+v found=%v", snap, found)
	}
	if found {
		t.Error("expected found=false on parse error")
	}
}

func TestFileSnapshotRepository_LoadSchemaVersionMismatch(t *testing.T) {
	repo := NewFileSnapshotRepository()
	workDir := t.TempDir()
	dir := filepath.Join(workDir, ".myT-x")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Write a snapshot with a future schema version that the current
	// reader does not understand.
	body := []byte(`{"schema_version":99,"work_dir":"x","saved_at":"2026-04-15T12:00:00Z","claude":null,"codex":null}`)
	path := filepath.Join(dir, "usage-dashboard.json")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, found, err := repo.Load(workDir)
	if err != nil {
		t.Fatalf("expected nil err for version mismatch (treated as miss), got: %v", err)
	}
	if found {
		t.Error("expected found=false for version mismatch")
	}
}

func TestSchemaVersionMismatchMessage(t *testing.T) {
	cases := []struct {
		name         string
		foundVersion int
		want         string
	}{
		{
			name:         "older snapshot",
			foundVersion: snapshotSchemaVersion - 1,
			want:         "[USAGE_DASHBOARD_DEBUG] snapshot schema version older than expected, ignoring cache",
		},
		{
			name:         "newer snapshot",
			foundVersion: snapshotSchemaVersion + 1,
			want:         "[USAGE_DASHBOARD_DEBUG] snapshot schema version newer than expected, ignoring cache",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := schemaVersionMismatchMessage(tc.foundVersion); got != tc.want {
				t.Errorf("schemaVersionMismatchMessage(%d) = %q, want %q", tc.foundVersion, got, tc.want)
			}
		})
	}
}

func TestFileSnapshotRepository_SaveCreatesDirectory(t *testing.T) {
	repo := NewFileSnapshotRepository()
	workDir := t.TempDir()
	// Confirm .myT-x does not exist yet.
	if _, err := os.Stat(filepath.Join(workDir, ".myT-x")); !os.IsNotExist(err) {
		t.Fatalf("precondition: .myT-x should not exist: err=%v", err)
	}
	if err := repo.Save(sampleSnapshot(workDir, time.Now().UTC())); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, ".myT-x")); err != nil {
		t.Errorf(".myT-x not created: %v", err)
	}
}

func TestFileSnapshotRepository_SaveRejectsEmptyWorkDir(t *testing.T) {
	repo := NewFileSnapshotRepository()
	snap := sampleSnapshot("", time.Now().UTC())
	snap.WorkDir = ""
	if err := repo.Save(snap); err == nil {
		t.Error("expected error for empty WorkDir")
	}
}

func TestFileSnapshotRepository_SaveAtomicLeavesNoTempFile(t *testing.T) {
	repo := NewFileSnapshotRepository()
	workDir := t.TempDir()
	if err := repo.Save(sampleSnapshot(workDir, time.Now().UTC())); err != nil {
		t.Fatalf("Save: %v", err)
	}
	// After a successful save, no leftover .tmp files should remain.
	entries, err := os.ReadDir(filepath.Join(workDir, ".myT-x"))
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("temp file leaked after Save: %s", e.Name())
		}
	}
}

func TestFileSnapshotRepository_SaveRenameFailureCleansTempFile(t *testing.T) {
	repo := NewFileSnapshotRepository()
	workDir := t.TempDir()
	blockedPath := filepath.Join(workDir, ".myT-x", "usage-dashboard.json")
	if err := os.MkdirAll(blockedPath, 0o755); err != nil {
		t.Fatalf("mkdir blocked target: %v", err)
	}

	err := repo.Save(sampleSnapshot(workDir, time.Now().UTC()))
	if err == nil {
		t.Fatal("expected Save to fail when target path is a directory")
	}

	entries, readErr := os.ReadDir(filepath.Join(workDir, ".myT-x"))
	if readErr != nil {
		t.Fatalf("readdir: %v", readErr)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatalf("temp file leaked after rename failure: %s", entry.Name())
		}
	}
}

func TestFileSnapshotRepository_SavedFileIsValidJSON(t *testing.T) {
	repo := NewFileSnapshotRepository()
	workDir := t.TempDir()
	if err := repo.Save(sampleSnapshot(workDir, time.Now().UTC())); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(workDir, ".myT-x", "usage-dashboard.json"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var probe map[string]any
	if err := json.Unmarshal(data, &probe); err != nil {
		t.Fatalf("on-disk file is not valid JSON: %v", err)
	}
	if probe["schema_version"] == nil {
		t.Error("schema_version field missing")
	}
}

func TestIsExpired(t *testing.T) {
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		savedAt time.Time
		ttl     time.Duration
		want    bool
	}{
		{"zero saved time is always expired", time.Time{}, time.Hour, true},
		{"within ttl", now.Add(-1 * time.Hour), 24 * time.Hour, false},
		{"exactly at ttl boundary is expired", now.Add(-24 * time.Hour), 24 * time.Hour, true},
		{"past ttl", now.Add(-25 * time.Hour), 24 * time.Hour, true},
		{"zero ttl falls back to default 24h", now.Add(-1 * time.Hour), 0, false},
		{"negative ttl falls back to default", now.Add(-1 * time.Hour), -time.Hour, false},
		{"zero ttl past default", now.Add(-25 * time.Hour), 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isExpired(tc.savedAt, now, tc.ttl); got != tc.want {
				t.Errorf("isExpired(%v, ttl=%v) = %v, want %v",
					tc.savedAt, tc.ttl, got, tc.want)
			}
		})
	}
}
