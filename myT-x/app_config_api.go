package main

import (
	"errors"
	"log/slog"
	"maps"
	"strings"
	"time"

	"myT-x/internal/config"
	"myT-x/internal/install"
	"myT-x/internal/tmux"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type configUpdatedEvent struct {
	Config             config.Config `json:"config"`
	Version            uint64        `json:"version"`
	UpdatedAtUnixMilli int64         `json:"updated_at_unix_milli"`
}

// ValidationRules contains validation parameters shared between backend and frontend.
type ValidationRules struct {
	MinOverrideNameLen int `json:"min_override_name_len"`
}

// GetConfig returns loaded config.
func (a *App) GetConfig() config.Config {
	return a.getConfigSnapshot()
}

// GetConfigAndFlushWarnings returns loaded config and emits any pending startup warnings.
func (a *App) GetConfigAndFlushWarnings() config.Config {
	a.flushPendingConfigLoadWarnings()
	return a.getConfigSnapshot()
}

func (a *App) flushPendingConfigLoadWarnings() {
	ctx := a.runtimeContext()
	if ctx == nil {
		return
	}
	if warning := a.consumePendingConfigLoadWarning(); warning != "" {
		a.emitRuntimeEventWithContext(ctx, "config:load-failed", map[string]string{
			"message": warning,
		})
	}
}

// SaveConfig validates and persists cfg to disk, then updates in-memory config.
// The config:updated event carries the normalized config (with defaults filled).
func (a *App) SaveConfig(cfg config.Config) error {
	event, err := a.saveConfigWithLock(cfg)
	if err != nil {
		return err
	}
	a.applyRuntimePaneEnvUpdate(event)
	a.applyRuntimeClaudeEnvUpdate(event)
	// Event emission intentionally happens outside cfgSaveMu.
	// Concurrent saves are ordered by Version, and frontend consumers must
	// treat the highest version as authoritative.
	a.emitRuntimeEvent("config:updated", event)
	return nil
}

// applyRuntimePaneEnvUpdate updates router pane_env defaults while preventing
// out-of-order writes from concurrent SaveConfig calls.
func (a *App) applyRuntimePaneEnvUpdate(event configUpdatedEvent) {
	router, guardErr := a.requireRouter()
	if guardErr != nil {
		slog.Warn("[WARN-CONFIG] skipped PaneEnv update: router unavailable", "error", guardErr)
		return
	}

	a.paneEnvUpdateMu.Lock()
	defer a.paneEnvUpdateMu.Unlock()

	// Defensive: use <= (not <) so that a duplicate event with the same version
	// is also rejected. Only a strictly newer version should trigger an update.
	if event.Version <= a.paneEnvAppliedVersion {
		slog.Debug("[DEBUG-CONFIG] skipped stale PaneEnv update", "received", event.Version, "applied", a.paneEnvAppliedVersion)
		return
	}

	router.UpdatePaneEnv(event.Config.PaneEnv)
	a.paneEnvAppliedVersion = event.Version
}

// applyRuntimeClaudeEnvUpdate updates router claude_env while preventing
// out-of-order writes from concurrent SaveConfig calls.
func (a *App) applyRuntimeClaudeEnvUpdate(event configUpdatedEvent) {
	router, guardErr := a.requireRouter()
	if guardErr != nil {
		slog.Warn("[WARN-CONFIG] skipped ClaudeEnv update: router unavailable", "error", guardErr)
		return
	}

	a.claudeEnvUpdateMu.Lock()
	defer a.claudeEnvUpdateMu.Unlock()

	if event.Version <= a.claudeEnvAppliedVersion {
		slog.Debug("[DEBUG-CONFIG] skipped stale ClaudeEnv update", "received", event.Version, "applied", a.claudeEnvAppliedVersion)
		return
	}

	var vars map[string]string
	if event.Config.ClaudeEnv != nil {
		vars = event.Config.ClaudeEnv.Vars
	}
	router.UpdateClaudeEnv(vars)
	a.claudeEnvAppliedVersion = event.Version
}

// saveConfigWithLock persists cfg, updates the in-memory snapshot, and bumps event version under cfgSaveMu.
func (a *App) saveConfigWithLock(cfg config.Config) (configUpdatedEvent, error) {
	a.cfgSaveMu.Lock()
	defer a.cfgSaveMu.Unlock()

	normalized, err := config.Save(a.configPath, cfg)
	if err != nil {
		return configUpdatedEvent{}, err
	}
	a.setConfigSnapshot(normalized)
	version := a.configEventVersion.Add(1)

	return configUpdatedEvent{
		Config:             config.Clone(normalized),
		Version:            version,
		UpdatedAtUnixMilli: time.Now().UnixMilli(),
	}, nil
}

// GetAllowedShells returns the list of allowed shell executables for UI dropdown.
func (a *App) GetAllowedShells() []string {
	return config.AllowedShellList()
}

// GetValidationRules returns frontend validation parameters shared with backend checks.
func (a *App) GetValidationRules() ValidationRules {
	return ValidationRules{
		MinOverrideNameLen: config.MinOverrideNameLen(),
	}
}

// GetClaudeEnvVarDescriptions returns known Claude Code environment variable
// names with Japanese descriptions for the frontend settings UI autocomplete.
// Returns a shallow copy to prevent callers from mutating the global map.
func (a *App) GetClaudeEnvVarDescriptions() map[string]string {
	cp := make(map[string]string, len(claudeEnvVarDescriptions))
	maps.Copy(cp, claudeEnvVarDescriptions)
	return cp
}

// ListSessions returns current session snapshots.
func (a *App) ListSessions() []tmux.SessionSnapshot {
	sessions, err := a.requireSessions()
	if err != nil {
		return nil
	}
	return sessions.Snapshot()
}

// PickSessionDirectory opens a directory picker for new session root.
func (a *App) PickSessionDirectory() (string, error) {
	ctx := a.runtimeContext()
	if ctx == nil {
		return "", errors.New("app context is not ready")
	}
	dir, err := runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{
		Title: "Select Session Root Directory",
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(dir), nil
}

// SetActiveSession sets current active session for status line and UI.
func (a *App) SetActiveSession(sessionName string) {
	name := a.setActiveSessionName(sessionName)
	a.emitRuntimeEvent("tmux:active-session", map[string]string{"name": name})
}

// GetActiveSession returns active session name.
func (a *App) GetActiveSession() string {
	return a.getActiveSessionName()
}

func (a *App) setActiveSessionName(sessionName string) string {
	normalized := strings.TrimSpace(sessionName)
	a.activeSessMu.Lock()
	a.activeSess = normalized
	a.activeSessMu.Unlock()
	return normalized
}

func (a *App) getActiveSessionName() string {
	a.activeSessMu.RLock()
	name := a.activeSess
	a.activeSessMu.RUnlock()
	return name
}

// DetachSession currently keeps process alive and only emits UI event.
func (a *App) DetachSession(sessionName string) {
	a.emitRuntimeEvent("tmux:session-detached", map[string]string{"name": sessionName})
}

// IsAgentTeamsAvailable reports whether Agent Teams can be started.
// Returns true when tmux CLI shim is installed and available on PATH.
func (a *App) IsAgentTeamsAvailable() bool {
	router, err := a.requireRouter()
	if err != nil {
		return false
	}
	return router.ShimAvailable()
}

// InstallTmuxShim triggers shim installer manually.
func (a *App) InstallTmuxShim() (install.ShimInstallResult, error) {
	result, err := ensureShimInstalledFn(a.workspace)
	if err != nil {
		return install.ShimInstallResult{}, err
	}
	if router, guardErr := a.requireRouter(); guardErr == nil {
		router.SetShimAvailable(true)
	}
	a.emitRuntimeEvent("tmux:shim-installed", result)
	return result, nil
}
