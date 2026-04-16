package main

import (
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"myT-x/internal/config"
)

// GetTaskSchedulerSettings returns the effective task scheduler settings,
// using persisted values when present and backend defaults otherwise. When the
// config is unset, the idle wait timeout defaults to 30 seconds.
// Wails-bound: called from the frontend task scheduler settings panel.
func (a *App) GetTaskSchedulerSettings() config.TaskSchedulerConfig {
	cfg := a.configState.Snapshot()
	if cfg.TaskScheduler == nil {
		return config.TaskSchedulerConfig{
			PreExecResetDelay:  defaultPreExecResetDelay,
			PreExecIdleTimeout: defaultPreExecIdleTimeout,
			PreExecTargetMode:  defaultPreExecTargetMode,
		}
	}
	return *cfg.TaskScheduler
}

// Defaults returned when no persisted config exists.
const (
	defaultPreExecResetDelay  = config.MinPreExecResetDelay                    // aligned with config sanitization and queue runtime defaults
	defaultPreExecIdleTimeout = config.DefaultPreExecIdleTimeout               // aligned with config sanitization and queue runtime defaults
	defaultPreExecTargetMode  = config.TaskSchedulerPreExecTargetModeTaskPanes // valid enum value
)

func taskSchedulerTargetModeError() error {
	modes := config.AllTaskSchedulerPreExecTargetModes()
	quotedModes := make([]string, 0, len(modes))
	for _, mode := range modes {
		quotedModes = append(quotedModes, fmt.Sprintf("%q", mode))
	}
	return fmt.Errorf("pre_exec_target_mode must be one of %s", strings.Join(quotedModes, ", "))
}

// validateTaskSchedulerSettingsInput treats the Wails-bound payload as untrusted
// input, then validates and normalizes it before persistence.
func validateTaskSchedulerSettingsInput(input config.TaskSchedulerConfig) (config.TaskSchedulerConfig, error) {
	settings := input
	if settings.PreExecResetDelay < config.MinPreExecResetDelay || settings.PreExecResetDelay > config.MaxPreExecResetDelay {
		return config.TaskSchedulerConfig{}, fmt.Errorf("pre_exec_reset_delay_s must be between %d and %d", config.MinPreExecResetDelay, config.MaxPreExecResetDelay)
	}
	if settings.PreExecIdleTimeout < config.MinPreExecIdleTimeout || settings.PreExecIdleTimeout > config.MaxPreExecIdleTimeout {
		return config.TaskSchedulerConfig{}, fmt.Errorf("pre_exec_idle_timeout_s must be between %d and %d", config.MinPreExecIdleTimeout, config.MaxPreExecIdleTimeout)
	}

	mode := strings.TrimSpace(string(settings.PreExecTargetMode))
	if mode == "" {
		slog.Debug("[TASK-SCHEDULER] empty pre-exec target mode normalized to default",
			"default", defaultPreExecTargetMode)
		mode = string(defaultPreExecTargetMode)
	}
	normalizedMode := config.TaskSchedulerPreExecTargetMode(mode)
	if !slices.Contains(config.AllTaskSchedulerPreExecTargetModes(), normalizedMode) {
		return config.TaskSchedulerConfig{}, taskSchedulerTargetModeError()
	}
	settings.PreExecTargetMode = normalizedMode

	if len(settings.MessageTemplates) > config.MaxMessageTemplates {
		return config.TaskSchedulerConfig{}, fmt.Errorf("message_templates: maximum %d templates allowed", config.MaxMessageTemplates)
	}

	// Defensive copy: validate and normalize templates without mutating
	// the caller's slice elements (work on our own copy).
	templates := make([]config.MessageTemplate, len(settings.MessageTemplates))
	seenNames := make(map[string]struct{}, len(settings.MessageTemplates))
	for i, tmpl := range settings.MessageTemplates {
		name := strings.TrimSpace(tmpl.Name)
		msg := strings.TrimSpace(tmpl.Message)
		if name == "" {
			return config.TaskSchedulerConfig{}, fmt.Errorf("message_templates[%d]: name is required", i)
		}
		if msg == "" {
			return config.TaskSchedulerConfig{}, fmt.Errorf("message_templates[%d]: message is required", i)
		}
		if len([]rune(name)) > config.MaxTemplateNameLen {
			return config.TaskSchedulerConfig{}, fmt.Errorf("message_templates[%d]: name exceeds maximum length of %d characters", i, config.MaxTemplateNameLen)
		}
		if len([]rune(msg)) > config.MaxTemplateMessageLen {
			return config.TaskSchedulerConfig{}, fmt.Errorf("message_templates[%d]: message exceeds maximum length of %d characters", i, config.MaxTemplateMessageLen)
		}
		if _, exists := seenNames[name]; exists {
			return config.TaskSchedulerConfig{}, fmt.Errorf("message_templates[%d]: duplicate template name %q", i, name)
		}
		seenNames[name] = struct{}{}
		templates[i] = config.MessageTemplate{Name: name, Message: msg}
	}
	settings.MessageTemplates = templates
	return settings, nil
}

// SaveTaskSchedulerSettings validates and persists task scheduler settings.
// Wails-bound: called from the frontend task scheduler settings panel.
func (a *App) SaveTaskSchedulerSettings(settings config.TaskSchedulerConfig) error {
	settings, err := validateTaskSchedulerSettingsInput(settings)
	if err != nil {
		return err
	}

	event, err := a.configState.Update(func(cfg *config.Config) {
		cfg.TaskScheduler = &settings
	})
	if err != nil {
		return err
	}
	a.emitConfigUpdatedEvent(event)
	return nil
}
