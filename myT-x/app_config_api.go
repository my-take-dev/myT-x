package main

import (
	"log/slog"
	"maps"
	"strings"

	"myT-x/internal/config"
)

// ValidationRules contains validation parameters shared between backend and frontend.
type ValidationRules struct {
	MinOverrideNameLen int `json:"min_override_name_len"`
	// Task scheduler validation boundaries.
	MinPreExecResetDelay  int `json:"min_pre_exec_reset_delay"`
	MaxPreExecResetDelay  int `json:"max_pre_exec_reset_delay"`
	MinPreExecIdleTimeout int `json:"min_pre_exec_idle_timeout"`
	MaxPreExecIdleTimeout int `json:"max_pre_exec_idle_timeout"`
	MaxMessageTemplates   int `json:"max_message_templates"`
	MaxTemplateNameLen    int `json:"max_template_name_len"`
	MaxTemplateMessageLen int `json:"max_template_message_len"`
}

// GetConfig returns loaded config.
func (a *App) GetConfig() config.Config {
	return a.configState.Snapshot()
}

// GetConfigAndFlushWarnings returns loaded config and emits any pending startup warnings.
func (a *App) GetConfigAndFlushWarnings() config.Config {
	a.flushPendingConfigLoadWarnings()
	return a.configState.Snapshot()
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
	event, err := a.configState.Save(cfg)
	if err != nil {
		return err
	}
	a.emitConfigUpdatedEvent(event)
	return nil
}

// ToggleViewerSidebarMode flips the persisted viewer sidebar mode using the
// latest in-memory config snapshot under the save lock to avoid stale overwrite.
func (a *App) ToggleViewerSidebarMode() error {
	event, err := a.configState.Update(func(cfg *config.Config) {
		if strings.TrimSpace(cfg.ViewerSidebarMode) == "docked" {
			cfg.ViewerSidebarMode = config.DefaultConfig().ViewerSidebarMode
		} else {
			cfg.ViewerSidebarMode = "docked"
		}
	})
	if err != nil {
		return err
	}
	a.emitConfigUpdatedEvent(event)
	return nil
}

func (a *App) emitConfigUpdatedEvent(event config.UpdatedEvent) {
	a.applyRuntimePaneEnvUpdate(event)
	a.applyRuntimeClaudeEnvUpdate(event)
	// Event emission intentionally happens outside the save lock.
	// Concurrent saves are ordered by Version, and frontend consumers must
	// treat the highest version as authoritative.
	a.emitRuntimeEvent("config:updated", event)
}

// applyRuntimePaneEnvUpdate updates router pane_env defaults while preventing
// out-of-order writes from concurrent SaveConfig calls.
func (a *App) applyRuntimePaneEnvUpdate(event config.UpdatedEvent) {
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
func (a *App) applyRuntimeClaudeEnvUpdate(event config.UpdatedEvent) {
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

// GetAllowedShells returns the list of allowed shell executables for UI dropdown.
func (a *App) GetAllowedShells() []string {
	return config.AllowedShellList()
}

// GetValidationRules returns frontend validation parameters shared with backend checks.
func (a *App) GetValidationRules() ValidationRules {
	return ValidationRules{
		MinOverrideNameLen:    config.MinOverrideNameLen(),
		MinPreExecResetDelay:  minPreExecResetDelay,
		MaxPreExecResetDelay:  maxPreExecResetDelay,
		MinPreExecIdleTimeout: minPreExecIdleTimeout,
		MaxPreExecIdleTimeout: maxPreExecIdleTimeout,
		MaxMessageTemplates:   maxMessageTemplates,
		MaxTemplateNameLen:    maxTemplateNameLen,
		MaxTemplateMessageLen: maxTemplateMessageLen,
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
