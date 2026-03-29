package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// newTestConfigPath returns a config.yaml path under t.TempDir with
// required env vars set for validateConfigPath.
func newTestConfigPath(t *testing.T) string {
	t.Helper()
	localAppData := t.TempDir()
	t.Setenv("LOCALAPPDATA", localAppData)
	t.Setenv("APPDATA", "")
	return filepath.Join(localAppData, "myT-x", "config.yaml")
}

// --- Snapshot tests (migrated from app_config_state_test.go) ---

func TestSnapshotReturnsIndependentCopy(t *testing.T) {
	s := NewStateService()
	base := DefaultConfig()
	s.SetSnapshot(base)

	snapshot := s.Snapshot()
	snapshot.Keys["snapshot-only"] = "value"
	snapshot.Worktree.SetupScripts = append(snapshot.Worktree.SetupScripts, "snapshot-script")

	latest := s.Snapshot()
	if _, exists := latest.Keys["snapshot-only"]; exists {
		t.Fatal("Snapshot returned shared map reference")
	}
	if len(latest.Worktree.SetupScripts) != len(base.Worktree.SetupScripts) {
		t.Fatal("Snapshot returned shared slice reference")
	}
}

func TestUnsafeSnapshotSharesReferenceFields(t *testing.T) {
	s := NewStateService()
	cfg := DefaultConfig()
	cfg.Keys = map[string]string{"base": "value"}
	cfg.Worktree.SetupScripts = []string{"setup-a"}
	s.SetSnapshot(cfg)

	readOnly := s.unsafeSnapshot()
	readOnly.Keys["shared-map"] = "mutated"
	readOnly.Worktree.SetupScripts[0] = "setup-mutated"

	latest := s.Snapshot()
	if latest.Keys["shared-map"] != "mutated" {
		t.Fatal("UnsafeSnapshot should expose shared map references")
	}
	if len(latest.Worktree.SetupScripts) == 0 || latest.Worktree.SetupScripts[0] != "setup-mutated" {
		t.Fatal("UnsafeSnapshot should expose shared slice references")
	}
}

func TestSnapshotConcurrency(t *testing.T) {
	s := NewStateService()
	s.SetSnapshot(DefaultConfig())

	const goroutines = 12
	const iterations = 200

	var wg sync.WaitGroup
	start := make(chan struct{})

	for i := range goroutines {
		wg.Go(func() {
			<-start

			for j := range iterations {
				cfg := s.Snapshot()
				cfg.Keys[fmt.Sprintf("goroutine-%d", i)] = fmt.Sprintf("%d", j)
				cfg.Worktree.SetupScripts = append(cfg.Worktree.SetupScripts, fmt.Sprintf("script-%d-%d", i, j))
				if i%2 == 0 {
					s.SetSnapshot(cfg)
					continue
				}
				_ = s.Snapshot()
			}
		})
	}

	close(start)
	wg.Wait()

	final := s.Snapshot()
	if final.Shell == "" {
		t.Fatal("config corruption detected: shell should not be empty")
	}
	if final.Keys == nil {
		t.Fatal("config corruption detected: keys should not be nil")
	}
	foundWriterKey := false
	for key := range final.Keys {
		if strings.HasPrefix(key, "goroutine-") {
			foundWriterKey = true
			break
		}
	}
	if !foundWriterKey {
		t.Fatal("config snapshot should include at least one writer key")
	}
	foundScript := false
	for _, script := range final.Worktree.SetupScripts {
		if script != "" {
			foundScript = true
			break
		}
	}
	if !foundScript {
		t.Fatal("config snapshot should include at least one writer-generated setup script")
	}
}

// --- Initialize tests ---

func TestInitializeSetsConfigPathAndSnapshot(t *testing.T) {
	s := NewStateService()
	cfg := DefaultConfig()
	cfg.Shell = "cmd.exe"

	s.Initialize("/test/config.yaml", cfg)

	if got := s.ConfigPath(); got != "/test/config.yaml" {
		t.Fatalf("ConfigPath() = %q, want %q", got, "/test/config.yaml")
	}
	snapshot := s.Snapshot()
	if snapshot.Shell != "cmd.exe" {
		t.Fatalf("Snapshot().Shell = %q, want %q", snapshot.Shell, "cmd.exe")
	}
}

func TestInitializeDeepCopiesConfig(t *testing.T) {
	s := NewStateService()
	cfg := DefaultConfig()
	cfg.Keys["init-key"] = "init-value"

	s.Initialize("/test/config.yaml", cfg)

	// Mutate original — must not affect service.
	cfg.Keys["after-init"] = "leaked"

	snapshot := s.Snapshot()
	if _, exists := snapshot.Keys["after-init"]; exists {
		t.Fatal("Initialize did not deep-copy config; mutation leaked")
	}
}

// --- Save tests ---

func TestSaveReturnsUpdatedEvent(t *testing.T) {
	configPath := newTestConfigPath(t)
	s := NewStateService()
	s.Initialize(configPath, DefaultConfig())

	cfg := DefaultConfig()
	cfg.Shell = "cmd.exe"
	event, err := s.Save(cfg)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if event.Version != 1 {
		t.Fatalf("Version = %d, want 1", event.Version)
	}
	if event.UpdatedAtUnixMilli <= 0 {
		t.Fatalf("UpdatedAtUnixMilli = %d, want > 0", event.UpdatedAtUnixMilli)
	}
	if event.Config.Shell != "cmd.exe" {
		t.Fatalf("event Config.Shell = %q, want %q", event.Config.Shell, "cmd.exe")
	}

	// Snapshot must reflect saved config.
	snapshot := s.Snapshot()
	if snapshot.Shell != "cmd.exe" {
		t.Fatalf("Snapshot().Shell = %q, want %q", snapshot.Shell, "cmd.exe")
	}
}

func TestSaveMonotonicVersions(t *testing.T) {
	configPath := newTestConfigPath(t)
	s := NewStateService()
	s.Initialize(configPath, DefaultConfig())

	cfg1 := DefaultConfig()
	cfg1.Shell = "cmd.exe"
	cfg2 := DefaultConfig()
	cfg2.Shell = "pwsh.exe"

	event1, err := s.Save(cfg1)
	if err != nil {
		t.Fatalf("Save(cfg1) error = %v", err)
	}
	event2, err := s.Save(cfg2)
	if err != nil {
		t.Fatalf("Save(cfg2) error = %v", err)
	}

	if event1.Version != 1 || event2.Version != 2 {
		t.Fatalf("versions = [%d %d], want [1 2]", event1.Version, event2.Version)
	}
}

func TestSaveDoesNotIncrementVersionOnError(t *testing.T) {
	s := NewStateService()
	s.Initialize("   ", DefaultConfig())
	s.SetEventVersion(7)

	if _, err := s.Save(DefaultConfig()); err == nil {
		t.Fatal("Save() expected error for empty config path")
	}
	if got := s.EventVersion(); got != 7 {
		t.Fatalf("EventVersion = %d, want 7", got)
	}
}

func TestSaveKeepsPreviousStateOnValidationError(t *testing.T) {
	configPath := newTestConfigPath(t)
	s := NewStateService()

	initial := DefaultConfig()
	initial.Shell = "cmd.exe"
	s.Initialize(configPath, initial)
	// Persist initial so Load can verify.
	if _, err := s.Save(initial); err != nil {
		t.Fatalf("Save(initial) error = %v", err)
	}

	invalid := initial
	invalid.Shell = "evil.exe"
	if _, err := s.Save(invalid); err == nil {
		t.Fatal("Save() expected validation error")
	}

	snapshot := s.Snapshot()
	if snapshot.Shell != "cmd.exe" {
		t.Fatalf("Shell after failed save = %q, want %q", snapshot.Shell, "cmd.exe")
	}

	// EventVersion must remain at 1 (from the initial Save) — the failed save
	// must not increment it.
	if got := s.EventVersion(); got != 1 {
		t.Fatalf("EventVersion after failed save = %d, want 1 (unchanged)", got)
	}

	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Shell != "cmd.exe" {
		t.Fatalf("persisted Shell after failed save = %q, want %q", loaded.Shell, "cmd.exe")
	}
}

func TestSaveEventConfigIsClone(t *testing.T) {
	configPath := newTestConfigPath(t)
	s := NewStateService()
	s.Initialize(configPath, DefaultConfig())

	event, err := s.Save(DefaultConfig())
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Mutate event payload — must not affect service state.
	event.Config.Keys["from-event"] = "value"
	after := s.Snapshot()
	if _, exists := after.Keys["from-event"]; exists {
		t.Fatal("mutating event payload should not mutate service config")
	}
}

func TestSavePersistsToDisk(t *testing.T) {
	configPath := newTestConfigPath(t)
	s := NewStateService()
	s.Initialize(configPath, DefaultConfig())

	cfg := DefaultConfig()
	cfg.Shell = "pwsh.exe"
	if _, err := s.Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file was written.
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Shell != "pwsh.exe" {
		t.Fatalf("loaded Shell = %q, want %q", loaded.Shell, "pwsh.exe")
	}
}

// --- Update tests ---

func TestUpdate(t *testing.T) {
	configPath := newTestConfigPath(t)
	s := NewStateService()
	initial := DefaultConfig()
	initial.Shell = "cmd.exe"
	initial.GlobalHotkey = "Ctrl+Alt+Y"
	s.Initialize(configPath, initial)

	event, err := s.Update(func(cfg *Config) {
		cfg.GlobalHotkey = "Ctrl+Shift+T"
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if event.Config.GlobalHotkey != "Ctrl+Shift+T" {
		t.Fatalf("GlobalHotkey = %q, want %q", event.Config.GlobalHotkey, "Ctrl+Shift+T")
	}
	// Other fields must be preserved.
	if event.Config.Shell != "cmd.exe" {
		t.Fatalf("Shell = %q, want %q", event.Config.Shell, "cmd.exe")
	}
	if event.Version != 1 {
		t.Fatalf("Version = %d, want 1", event.Version)
	}

	// Verify persisted state.
	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.GlobalHotkey != "Ctrl+Shift+T" {
		t.Fatalf("persisted GlobalHotkey = %q, want %q", loaded.GlobalHotkey, "Ctrl+Shift+T")
	}
	if loaded.Shell != "cmd.exe" {
		t.Fatalf("persisted Shell = %q, want %q", loaded.Shell, "cmd.exe")
	}
}

func TestUpdateFailsWithInvalidConfigPath(t *testing.T) {
	s := NewStateService()
	s.Initialize("   ", DefaultConfig())

	_, err := s.Update(func(cfg *Config) {
		cfg.GlobalHotkey = "Ctrl+T"
	})
	if err == nil {
		t.Fatal("Update() expected error for invalid config path")
	}
}

// --- Save concurrency test ---

func TestSaveConcurrency(t *testing.T) {
	configPath := newTestConfigPath(t)
	s := NewStateService()
	s.Initialize(configPath, DefaultConfig())

	const goroutines = 8
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, goroutines)
	versions := make([]uint64, goroutines)

	for i := range goroutines {
		wg.Go(func() {
			<-start
			cfg := DefaultConfig()
			cfg.GlobalHotkey = fmt.Sprintf("Ctrl+Alt+%d", i)
			event, err := s.Save(cfg)
			errs[i] = err
			if err == nil {
				versions[i] = event.Version
			}
		})
	}

	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("Save goroutine %d error = %v", i, err)
		}
	}
	// All versions must be unique (saveMu serializes).
	seen := make(map[uint64]bool)
	for _, v := range versions {
		if seen[v] {
			t.Fatalf("duplicate version %d: saveMu did not serialize", v)
		}
		seen[v] = true
	}
	if got := s.EventVersion(); got != uint64(goroutines) {
		t.Fatalf("EventVersion = %d, want %d", got, goroutines)
	}
}

// --- Initialize guard test ---

func TestInitializePanicsOnDoubleCall(t *testing.T) {
	s := NewStateService()
	s.Initialize("/test/config.yaml", DefaultConfig())

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on double Initialize call")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "called more than once") {
			t.Fatalf("unexpected panic value: %v", r)
		}
	}()
	s.Initialize("/other/config.yaml", DefaultConfig())
}

// --- EventVersion tests ---

func TestSetEventVersion(t *testing.T) {
	s := NewStateService()
	s.SetEventVersion(42)
	if got := s.EventVersion(); got != 42 {
		t.Fatalf("EventVersion() = %d, want 42", got)
	}
}

// --- Field count guard ---

func TestUpdatedEventFieldCount(t *testing.T) {
	if got := reflect.TypeFor[UpdatedEvent]().NumField(); got != 3 {
		t.Fatalf("UpdatedEvent field count = %d, want 3; update emit payload and tests for new fields", got)
	}
}

// --- ConfigPath tests ---

func TestConfigPathBeforeInitialize(t *testing.T) {
	s := NewStateService()
	if got := s.ConfigPath(); got != "" {
		t.Fatalf("ConfigPath() before Initialize = %q, want empty", got)
	}
}
