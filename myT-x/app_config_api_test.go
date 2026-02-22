package main

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"myT-x/internal/config"
	"myT-x/internal/install"
	"myT-x/internal/tmux"
)

// NOTE: This file overrides package-level function variables
// (runtimeEventsEmitFn, ensureShimInstalledFn). Do not use t.Parallel() here.

func newConfigPathForAPITest(t *testing.T, fileName string) string {
	t.Helper()
	localAppData := t.TempDir()
	t.Setenv("LOCALAPPDATA", localAppData)
	t.Setenv("APPDATA", "")

	defaultPath := config.DefaultPath()
	return filepath.Join(filepath.Dir(defaultPath), fileName)
}

func TestSaveConfigEmitsUpdatedConfigEvent(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.configPath = newConfigPathForAPITest(t, "config.yaml")
	app.setConfigSnapshot(config.DefaultConfig())

	eventCount := 0
	var eventName string
	var eventPayload configUpdatedEvent
	runtimeEventsEmitFn = func(_ context.Context, name string, data ...any) {
		eventCount++
		eventName = name
		if len(data) == 0 {
			return
		}
		payload, ok := data[0].(configUpdatedEvent)
		if ok {
			eventPayload = payload
		}
	}

	if err := app.SaveConfig(config.Config{}); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	got := app.GetConfig()
	if got.Shell != config.DefaultConfig().Shell {
		t.Fatalf("saved shell = %q, want %q", got.Shell, config.DefaultConfig().Shell)
	}
	if eventCount != 1 {
		t.Fatalf("event count = %d, want 1", eventCount)
	}
	if eventName != "config:updated" {
		t.Fatalf("event name = %q, want %q", eventName, "config:updated")
	}
	if eventPayload.Config.Shell != got.Shell {
		t.Fatalf("event payload shell = %q, want %q", eventPayload.Config.Shell, got.Shell)
	}
	if eventPayload.Version != 1 {
		t.Fatalf("event version = %d, want 1", eventPayload.Version)
	}
	if eventPayload.UpdatedAtUnixMilli <= 0 {
		t.Fatalf("event updated_at_unix_milli = %d, want > 0", eventPayload.UpdatedAtUnixMilli)
	}

	// Ensure event payload is a clone and does not mutate app state.
	eventPayload.Config.Keys["from-event"] = "value"
	after := app.GetConfig()
	if _, exists := after.Keys["from-event"]; exists {
		t.Fatal("mutating event payload should not mutate app config")
	}
}

func TestSaveConfigEmitsMonotonicEventVersion(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.configPath = newConfigPathForAPITest(t, "config.yaml")
	app.setConfigSnapshot(config.DefaultConfig())

	var versions []uint64
	runtimeEventsEmitFn = func(_ context.Context, name string, data ...any) {
		if name != "config:updated" || len(data) == 0 {
			return
		}
		payload, ok := data[0].(configUpdatedEvent)
		if !ok {
			t.Fatalf("unexpected payload type: %T", data[0])
		}
		versions = append(versions, payload.Version)
	}

	cfg1 := config.DefaultConfig()
	cfg1.Shell = "cmd.exe"
	cfg2 := config.DefaultConfig()
	cfg2.Shell = "pwsh.exe"

	if err := app.SaveConfig(cfg1); err != nil {
		t.Fatalf("SaveConfig(cfg1) error = %v", err)
	}
	if err := app.SaveConfig(cfg2); err != nil {
		t.Fatalf("SaveConfig(cfg2) error = %v", err)
	}

	if len(versions) != 2 {
		t.Fatalf("version count = %d, want 2", len(versions))
	}
	if versions[0] != 1 || versions[1] != 2 {
		t.Fatalf("versions = %v, want [1 2]", versions)
	}
}

func TestSaveConfigKeepsPreviousStateOnValidationError(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.configPath = newConfigPathForAPITest(t, "config.yaml")

	events := 0
	runtimeEventsEmitFn = func(_ context.Context, _ string, _ ...any) {
		events++
	}

	initial := config.DefaultConfig()
	initial.Shell = "cmd.exe"
	if err := app.SaveConfig(initial); err != nil {
		t.Fatalf("SaveConfig(initial) error = %v", err)
	}

	events = 0
	invalid := initial
	invalid.Shell = "evil.exe"

	if err := app.SaveConfig(invalid); err == nil {
		t.Fatal("SaveConfig() expected validation error")
	}
	if events != 0 {
		t.Fatalf("event count after failed save = %d, want 0", events)
	}

	got := app.GetConfig()
	if got.Shell != initial.Shell {
		t.Fatalf("config shell after failed save = %q, want %q", got.Shell, initial.Shell)
	}

	loaded, err := config.Load(app.configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Shell != initial.Shell {
		t.Fatalf("persisted shell after failed save = %q, want %q", loaded.Shell, initial.Shell)
	}
}

func TestSessionAPIsEmitEventsThroughRuntimeEventsEmitFn(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	origInstall := ensureShimInstalledFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
		ensureShimInstalledFn = origInstall
	})

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.workspace = t.TempDir()

	var mu sync.Mutex
	events := make([]string, 0, 3)
	runtimeEventsEmitFn = func(_ context.Context, name string, _ ...any) {
		mu.Lock()
		events = append(events, name)
		mu.Unlock()
	}

	wantInstallResult := install.ShimInstallResult{InstalledPath: filepath.Join(app.workspace, "tmux.exe")}
	ensureShimInstalledFn = func(string) (install.ShimInstallResult, error) {
		return wantInstallResult, nil
	}

	app.SetActiveSession("session-a")
	app.DetachSession("session-a")
	gotInstallResult, err := app.InstallTmuxShim()
	if err != nil {
		t.Fatalf("InstallTmuxShim() error = %v", err)
	}
	if gotInstallResult != wantInstallResult {
		t.Fatalf("InstallTmuxShim() result = %+v, want %+v", gotInstallResult, wantInstallResult)
	}

	mu.Lock()
	gotEvents := append([]string(nil), events...)
	mu.Unlock()
	wantEvents := []string{
		"tmux:active-session",
		"tmux:session-detached",
		"tmux:shim-installed",
	}
	if len(gotEvents) != len(wantEvents) {
		t.Fatalf("event count = %d, want %d (%v)", len(gotEvents), len(wantEvents), gotEvents)
	}
	for i := range wantEvents {
		if gotEvents[i] != wantEvents[i] {
			t.Fatalf("event[%d] = %q, want %q", i, gotEvents[i], wantEvents[i])
		}
	}
}

func TestInstallTmuxShimDoesNotEmitOnError(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	origInstall := ensureShimInstalledFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
		ensureShimInstalledFn = origInstall
	})

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.workspace = t.TempDir()

	eventCount := 0
	runtimeEventsEmitFn = func(_ context.Context, _ string, _ ...any) {
		eventCount++
	}
	ensureShimInstalledFn = func(string) (install.ShimInstallResult, error) {
		return install.ShimInstallResult{}, errors.New("install failed")
	}

	if _, err := app.InstallTmuxShim(); err == nil {
		t.Fatal("InstallTmuxShim() expected error")
	}
	if eventCount != 0 {
		t.Fatalf("event count = %d, want 0", eventCount)
	}
}

func TestSaveConfigRejectsEmptyConfigPath(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.configPath = "   "

	eventCount := 0
	runtimeEventsEmitFn = func(_ context.Context, _ string, _ ...any) {
		eventCount++
	}

	if err := app.SaveConfig(config.DefaultConfig()); err == nil {
		t.Fatal("SaveConfig() expected error for empty config path")
	}
	if eventCount != 0 {
		t.Fatalf("event count = %d, want 0", eventCount)
	}
}

func TestSaveConfigWithLockDoesNotIncrementEventVersionOnSaveError(t *testing.T) {
	app := NewApp()
	app.configPath = "   "
	app.configEventVersion.Store(7)
	app.setConfigSnapshot(config.DefaultConfig())

	if _, err := app.saveConfigWithLock(config.DefaultConfig()); err == nil {
		t.Fatal("saveConfigWithLock() expected error")
	}
	if got := app.configEventVersion.Load(); got != 7 {
		t.Fatalf("configEventVersion = %d, want 7", got)
	}
}

func TestApplyRuntimeClaudeEnvUpdateRouterNil(t *testing.T) {
	app := NewApp()
	// router is nil — must not panic.
	app.applyRuntimeClaudeEnvUpdate(configUpdatedEvent{
		Config:  config.DefaultConfig(),
		Version: 1,
	})
	if app.claudeEnvAppliedVersion != 0 {
		t.Fatalf("claudeEnvAppliedVersion = %d, want 0 (should not update when router is nil)", app.claudeEnvAppliedVersion)
	}
}

func TestApplyRuntimeClaudeEnvUpdateSkipsStaleVersion(t *testing.T) {
	app := NewApp()
	app.router = tmux.NewCommandRouter(nil, nil, tmux.RouterOptions{})

	newerCfg := config.DefaultConfig()
	newerCfg.ClaudeEnv = &config.ClaudeEnvConfig{Vars: map[string]string{"A": "new"}}
	olderCfg := config.DefaultConfig()
	olderCfg.ClaudeEnv = &config.ClaudeEnvConfig{Vars: map[string]string{"A": "old"}}

	// Apply version 2 first, then stale version 1 — version 1 must be rejected.
	app.applyRuntimeClaudeEnvUpdate(configUpdatedEvent{
		Config:  newerCfg,
		Version: 2,
	})
	app.applyRuntimeClaudeEnvUpdate(configUpdatedEvent{
		Config:  olderCfg,
		Version: 1,
	})

	if got := app.claudeEnvAppliedVersion; got != 2 {
		t.Fatalf("claudeEnvAppliedVersion = %d, want 2", got)
	}
	// Verify actual router ClaudeEnv reflects version 2 (not stale version 1).
	if env := app.router.ClaudeEnvSnapshot(); env["A"] != "new" {
		t.Fatalf("router ClaudeEnv[A] = %q, want %q (stale version was applied)", env["A"], "new")
	}

	// Apply version 3 to confirm forward progress works.
	v3Cfg := config.DefaultConfig()
	v3Cfg.ClaudeEnv = &config.ClaudeEnvConfig{Vars: map[string]string{"B": "v3"}}
	app.applyRuntimeClaudeEnvUpdate(configUpdatedEvent{
		Config:  v3Cfg,
		Version: 3,
	})
	if got := app.claudeEnvAppliedVersion; got != 3 {
		t.Fatalf("claudeEnvAppliedVersion after newer update = %d, want 3", got)
	}
	// Verify router ClaudeEnv reflects version 3 content.
	env3 := app.router.ClaudeEnvSnapshot()
	if env3["B"] != "v3" {
		t.Fatalf("router ClaudeEnv[B] = %q, want %q", env3["B"], "v3")
	}
	if _, exists := env3["A"]; exists {
		t.Fatal("router ClaudeEnv still contains key A from version 2 after version 3 overwrite")
	}

	// Apply duplicate version 3 — must be rejected (defensive <= check).
	dupCfg := config.DefaultConfig()
	dupCfg.ClaudeEnv = &config.ClaudeEnvConfig{Vars: map[string]string{"B": "dup"}}
	app.applyRuntimeClaudeEnvUpdate(configUpdatedEvent{
		Config:  dupCfg,
		Version: 3,
	})
	if env := app.router.ClaudeEnvSnapshot(); env["B"] != "v3" {
		t.Fatalf("router ClaudeEnv[B] = %q after duplicate version, want %q", env["B"], "v3")
	}
}

func TestApplyRuntimePaneEnvUpdateRouterNil(t *testing.T) {
	app := NewApp()
	// router is nil — must not panic.
	app.applyRuntimePaneEnvUpdate(configUpdatedEvent{
		Config:  config.DefaultConfig(),
		Version: 1,
	})
	if app.paneEnvAppliedVersion != 0 {
		t.Fatalf("paneEnvAppliedVersion = %d, want 0 (should not update when router is nil)", app.paneEnvAppliedVersion)
	}
}

func TestApplyRuntimePaneEnvUpdateSkipsStaleVersion(t *testing.T) {
	app := NewApp()
	app.router = tmux.NewCommandRouter(nil, nil, tmux.RouterOptions{})

	newerCfg := config.DefaultConfig()
	newerCfg.PaneEnv = map[string]string{"A": "new"}
	olderCfg := config.DefaultConfig()
	olderCfg.PaneEnv = map[string]string{"A": "old"}

	// Apply version 2 first, then stale version 1 — version 1 must be rejected.
	app.applyRuntimePaneEnvUpdate(configUpdatedEvent{
		Config:  newerCfg,
		Version: 2,
	})
	app.applyRuntimePaneEnvUpdate(configUpdatedEvent{
		Config:  olderCfg,
		Version: 1,
	})

	if got := app.paneEnvAppliedVersion; got != 2 {
		t.Fatalf("paneEnvAppliedVersion = %d, want 2", got)
	}
	// Verify actual router PaneEnv reflects version 2 (not stale version 1).
	if env := app.router.PaneEnvSnapshot(); env["A"] != "new" {
		t.Fatalf("router PaneEnv[A] = %q, want %q (stale version was applied)", env["A"], "new")
	}

	// Apply version 3 to confirm forward progress works.
	v3Cfg := config.DefaultConfig()
	v3Cfg.PaneEnv = map[string]string{"B": "v3"}
	app.applyRuntimePaneEnvUpdate(configUpdatedEvent{
		Config:  v3Cfg,
		Version: 3,
	})
	if got := app.paneEnvAppliedVersion; got != 3 {
		t.Fatalf("paneEnvAppliedVersion after newer update = %d, want 3", got)
	}
	// Verify router PaneEnv reflects version 3 content.
	env3 := app.router.PaneEnvSnapshot()
	if env3["B"] != "v3" {
		t.Fatalf("router PaneEnv[B] = %q, want %q", env3["B"], "v3")
	}
	if _, exists := env3["A"]; exists {
		t.Fatal("router PaneEnv still contains key A from version 2 after version 3 overwrite")
	}

	// Apply duplicate version 3 — must be rejected (defensive <= check).
	dupCfg := config.DefaultConfig()
	dupCfg.PaneEnv = map[string]string{"B": "dup"}
	app.applyRuntimePaneEnvUpdate(configUpdatedEvent{
		Config:  dupCfg,
		Version: 3,
	})
	if env := app.router.PaneEnvSnapshot(); env["B"] != "v3" {
		t.Fatalf("router PaneEnv[B] = %q after duplicate version, want %q", env["B"], "v3")
	}
}

func TestGetAllowedShells(t *testing.T) {
	app := NewApp()
	got := app.GetAllowedShells()
	want := config.AllowedShellList()
	if len(got) != len(want) {
		t.Fatalf("GetAllowedShells() length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("GetAllowedShells()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestListSessions(t *testing.T) {
	t.Run("returns nil when session manager is unavailable", func(t *testing.T) {
		app := NewApp()
		app.sessions = nil
		if got := app.ListSessions(); got != nil {
			t.Fatalf("ListSessions() = %v, want nil", got)
		}
	})

	t.Run("returns snapshots from session manager", func(t *testing.T) {
		app := NewApp()
		app.sessions = tmux.NewSessionManager()
		if _, _, err := app.sessions.CreateSession("alpha", "0", 120, 40); err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}

		got := app.ListSessions()
		if len(got) != 1 {
			t.Fatalf("ListSessions() length = %d, want 1", len(got))
		}
		if got[0].Name != "alpha" {
			t.Fatalf("ListSessions()[0].Name = %q, want %q", got[0].Name, "alpha")
		}
	})
}

func TestIsAgentTeamsAvailable(t *testing.T) {
	t.Run("returns false when router is unavailable", func(t *testing.T) {
		app := NewApp()
		app.router = nil
		if app.IsAgentTeamsAvailable() {
			t.Fatal("IsAgentTeamsAvailable() should be false when router is nil")
		}
	})

	t.Run("returns router shim availability", func(t *testing.T) {
		app := NewApp()
		app.router = tmux.NewCommandRouter(nil, nil, tmux.RouterOptions{ShimAvailable: true})
		if !app.IsAgentTeamsAvailable() {
			t.Fatal("IsAgentTeamsAvailable() should be true when shim is available")
		}
	})
}

func TestGetValidationRules(t *testing.T) {
	app := NewApp()
	rules := app.GetValidationRules()

	if got := reflect.TypeFor[ValidationRules]().NumField(); got != 1 {
		t.Fatalf("ValidationRules field count = %d, want 1; update TestGetValidationRules for new fields", got)
	}
	if rules.MinOverrideNameLen != config.MinOverrideNameLen() {
		t.Fatalf(
			"min_override_name_len = %d, want %d",
			rules.MinOverrideNameLen,
			config.MinOverrideNameLen(),
		)
	}
}

// TestGetClaudeEnvVarDescriptionsMutationSafety verifies that the map returned
// by GetClaudeEnvVarDescriptions is a defensive copy. Mutating the returned map
// must not affect subsequent calls — callers cannot corrupt the global descriptions.
func TestGetClaudeEnvVarDescriptionsMutationSafety(t *testing.T) {
	app := NewApp()

	// First call: get a reference and capture the original size.
	first := app.GetClaudeEnvVarDescriptions()
	if len(first) == 0 {
		t.Fatal("GetClaudeEnvVarDescriptions() returned empty map; expected non-empty global descriptions")
	}
	originalLen := len(first)

	// Pick an existing key to mutate and verify the original value is known.
	var existingKey string
	var originalValue string
	for k, v := range first {
		existingKey = k
		originalValue = v
		break
	}

	// Mutate the returned map: overwrite an existing key and add a new key.
	first[existingKey] = "MUTATED_VALUE"
	first["INJECTED_BY_CALLER"] = "injected"

	// Second call: the global map must be unaffected by the mutations above.
	second := app.GetClaudeEnvVarDescriptions()
	if len(second) != originalLen {
		t.Fatalf("GetClaudeEnvVarDescriptions() length after mutation = %d, want %d (original)", len(second), originalLen)
	}
	if got := second[existingKey]; got != originalValue {
		t.Fatalf("GetClaudeEnvVarDescriptions()[%q] = %q after mutation, want %q (original)", existingKey, got, originalValue)
	}
	if _, injected := second["INJECTED_BY_CALLER"]; injected {
		t.Fatal("GetClaudeEnvVarDescriptions() contains INJECTED_BY_CALLER; defensive copy failed")
	}
}

func TestConfigEventFieldCounts(t *testing.T) {
	if got := reflect.TypeFor[configUpdatedEvent]().NumField(); got != 3 {
		t.Fatalf("configUpdatedEvent field count = %d, want 3; update emit payload and tests for new fields", got)
	}
}

func TestGetConfigAndFlushWarningsEmitsPendingConfigLoadWarningOnce(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.setConfigSnapshot(config.DefaultConfig())
	app.addPendingConfigLoadWarning("failed to load config at startup")

	eventCount := 0
	lastEvent := ""
	var lastPayload map[string]string
	runtimeEventsEmitFn = func(_ context.Context, name string, data ...any) {
		eventCount++
		lastEvent = name
		if len(data) == 0 {
			return
		}
		payload, ok := data[0].(map[string]string)
		if ok {
			lastPayload = payload
		}
	}

	_ = app.GetConfigAndFlushWarnings()
	_ = app.GetConfigAndFlushWarnings()

	if eventCount != 1 {
		t.Fatalf("event count = %d, want 1", eventCount)
	}
	if lastEvent != "config:load-failed" {
		t.Fatalf("event name = %q, want %q", lastEvent, "config:load-failed")
	}
	if message := lastPayload["message"]; message != "failed to load config at startup" {
		t.Fatalf("warning message = %q, want %q", message, "failed to load config at startup")
	}
}

func TestGetConfigDoesNotFlushPendingWarnings(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.setConfigSnapshot(config.DefaultConfig())
	app.addPendingConfigLoadWarning("warning-to-flush-later")

	eventCount := 0
	runtimeEventsEmitFn = func(_ context.Context, name string, _ ...any) {
		if name == "config:load-failed" {
			eventCount++
		}
	}

	_ = app.GetConfig()
	if eventCount != 0 {
		t.Fatalf("GetConfig() emitted %d warning events, want 0", eventCount)
	}
	if warning := app.consumePendingConfigLoadWarning(); warning != "warning-to-flush-later" {
		t.Fatalf("consumePendingConfigLoadWarning() after GetConfig = %q, want %q", warning, "warning-to-flush-later")
	}
	app.addPendingConfigLoadWarning("warning-to-flush-later")

	_ = app.GetConfigAndFlushWarnings()
	if eventCount != 1 {
		t.Fatalf("GetConfigAndFlushWarnings() warning event count = %d, want 1", eventCount)
	}
}

func TestSetPendingConfigLoadWarningIgnoresWhitespaceOnlyInput(t *testing.T) {
	app := NewApp()
	app.addPendingConfigLoadWarning(" \t \n ")

	if warning := app.consumePendingConfigLoadWarning(); warning != "" {
		t.Fatalf("consumePendingConfigLoadWarning() = %q, want empty", warning)
	}
}

func TestGetConfigAndFlushWarningsEmitsCombinedPendingConfigLoadWarnings(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.setConfigSnapshot(config.DefaultConfig())
	app.addPendingConfigLoadWarning("failed to load config at startup")
	app.addPendingConfigLoadWarning("failed to start pipe server at startup")

	eventCount := 0
	lastEvent := ""
	var lastPayload map[string]string
	runtimeEventsEmitFn = func(_ context.Context, name string, data ...any) {
		eventCount++
		lastEvent = name
		if len(data) == 0 {
			return
		}
		payload, ok := data[0].(map[string]string)
		if ok {
			lastPayload = payload
		}
	}

	_ = app.GetConfigAndFlushWarnings()
	_ = app.GetConfigAndFlushWarnings()

	if eventCount != 1 {
		t.Fatalf("event count = %d, want 1", eventCount)
	}
	if lastEvent != "config:load-failed" {
		t.Fatalf("event name = %q, want %q", lastEvent, "config:load-failed")
	}
	want := "failed to load config at startup\nfailed to start pipe server at startup"
	if message := lastPayload["message"]; message != want {
		t.Fatalf("warning message = %q, want %q", message, want)
	}
}

func TestConfigAndSessionAPIsSkipRuntimeEventsWhenContextIsNil(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	origInstall := ensureShimInstalledFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
		ensureShimInstalledFn = origInstall
	})

	app := NewApp()
	app.configPath = newConfigPathForAPITest(t, "config.yaml")
	app.workspace = t.TempDir()

	eventCount := 0
	runtimeEventsEmitFn = func(_ context.Context, _ string, _ ...any) {
		eventCount++
	}
	ensureShimInstalledFn = func(string) (install.ShimInstallResult, error) {
		return install.ShimInstallResult{InstalledPath: filepath.Join(app.workspace, "tmux.exe")}, nil
	}

	if err := app.SaveConfig(config.DefaultConfig()); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
	app.SetActiveSession("session-a")
	app.DetachSession("session-a")
	if _, err := app.InstallTmuxShim(); err != nil {
		t.Fatalf("InstallTmuxShim() error = %v", err)
	}

	if eventCount != 0 {
		t.Fatalf("event count = %d, want 0", eventCount)
	}
}

func TestPickSessionDirectoryRequiresRuntimeContext(t *testing.T) {
	app := NewApp()
	app.setRuntimeContext(nil)

	if _, err := app.PickSessionDirectory(); err == nil {
		t.Fatal("PickSessionDirectory() expected context-not-ready error")
	}
}

func TestSetActiveSessionUpdatesStateAndEmitsTrimmedName(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	app := NewApp()
	app.setRuntimeContext(context.Background())

	eventCount := 0
	emittedName := ""
	runtimeEventsEmitFn = func(_ context.Context, name string, data ...any) {
		if name != "tmux:active-session" {
			return
		}
		eventCount++
		if len(data) == 0 {
			return
		}
		payload, ok := data[0].(map[string]string)
		if !ok {
			return
		}
		emittedName = payload["name"]
	}

	app.SetActiveSession("  session-a  ")

	if got := app.GetActiveSession(); got != "session-a" {
		t.Fatalf("GetActiveSession() = %q, want %q", got, "session-a")
	}
	if emittedName != "session-a" {
		t.Fatalf("emitted active session = %q, want %q", emittedName, "session-a")
	}
	if eventCount != 1 {
		t.Fatalf("event count = %d, want 1", eventCount)
	}
}

func TestSetActiveSessionTrimsWhitespaceOnlyToEmpty(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})
	runtimeEventsEmitFn = func(_ context.Context, _ string, _ ...any) {}

	app := NewApp()
	app.SetActiveSession("   ")
	if got := app.GetActiveSession(); got != "" {
		t.Fatalf("GetActiveSession() = %q, want empty string", got)
	}
}

func TestSaveConfigSerializesConcurrentUpdates(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.configPath = newConfigPathForAPITest(t, "config.yaml")

	enterFirstEvent := make(chan struct{})
	releaseFirstEvent := make(chan struct{})
	secondEventEntered := make(chan struct{})
	var eventCount atomic.Int32

	runtimeEventsEmitFn = func(_ context.Context, _ string, _ ...any) {
		current := eventCount.Add(1)
		if current == 1 {
			close(enterFirstEvent)
			<-releaseFirstEvent
			return
		}
		if current == 2 {
			close(secondEventEntered)
		}
	}

	cfg1 := config.DefaultConfig()
	cfg1.Shell = "cmd.exe"
	cfg2 := config.DefaultConfig()
	cfg2.Shell = "pwsh.exe"

	firstDone := make(chan error, 1)
	secondDone := make(chan error, 1)
	secondStarted := make(chan struct{})

	go func() {
		firstDone <- app.SaveConfig(cfg1)
	}()

	select {
	case <-enterFirstEvent:
	case <-time.After(5 * time.Second):
		t.Fatal("first SaveConfig did not reach event emission")
	}

	go func() {
		close(secondStarted)
		secondDone <- app.SaveConfig(cfg2)
	}()

	select {
	case <-secondStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("second SaveConfig did not start")
	}

	select {
	case <-secondEventEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("second SaveConfig did not reach event emission")
	}

	select {
	case err := <-secondDone:
		if err != nil {
			t.Fatalf("second SaveConfig() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("second SaveConfig should complete while first event handler is blocked")
	}

	if got := eventCount.Load(); got != 2 {
		t.Fatalf("event count before releasing first event = %d, want 2", got)
	}

	if got := app.GetConfig().Shell; got != cfg2.Shell {
		t.Fatalf("final shell before releasing first event = %q, want %q", got, cfg2.Shell)
	}

	close(releaseFirstEvent)

	select {
	case err := <-firstDone:
		if err != nil {
			t.Fatalf("first SaveConfig() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("first SaveConfig timed out")
	}
}
