package main

import (
	"errors"
	"fmt"
	"strings"

	"myT-x/internal/config"
)

// GetTaskSchedulerSettings returns the persisted task scheduler settings.
// Wails-bound: called from the frontend task scheduler settings panel.
func (a *App) GetTaskSchedulerSettings() config.TaskSchedulerConfig {
	cfg := a.configState.Snapshot()
	if cfg.TaskScheduler == nil {
		return config.TaskSchedulerConfig{}
	}
	return *cfg.TaskScheduler
}

// Validation limits for task scheduler settings.
// Frontend shares these via GetValidationRules().
const (
	minPreExecResetDelay  = 0
	maxPreExecResetDelay  = 60
	minPreExecIdleTimeout = 10
	maxPreExecIdleTimeout = 600
	maxMessageTemplates   = 50
	maxTemplateNameLen    = 100
	maxTemplateMessageLen = 5000
)

// SaveTaskSchedulerSettings validates and persists task scheduler settings.
// Wails-bound: called from the frontend task scheduler settings panel.
func (a *App) SaveTaskSchedulerSettings(settings config.TaskSchedulerConfig) error {
	if settings.PreExecResetDelay < minPreExecResetDelay || settings.PreExecResetDelay > maxPreExecResetDelay {
		return fmt.Errorf("pre_exec_reset_delay_s must be between %d and %d", minPreExecResetDelay, maxPreExecResetDelay)
	}
	if settings.PreExecIdleTimeout < minPreExecIdleTimeout || settings.PreExecIdleTimeout > maxPreExecIdleTimeout {
		return fmt.Errorf("pre_exec_idle_timeout_s must be between %d and %d", minPreExecIdleTimeout, maxPreExecIdleTimeout)
	}

	mode := strings.TrimSpace(settings.PreExecTargetMode)
	if mode != "" && mode != "task_panes" && mode != "all_panes" {
		return errors.New("pre_exec_target_mode must be 'task_panes' or 'all_panes'")
	}
	settings.PreExecTargetMode = mode

	if len(settings.MessageTemplates) > maxMessageTemplates {
		return fmt.Errorf("message_templates: maximum %d templates allowed", maxMessageTemplates)
	}

	// Defensive copy: validate and normalize templates without mutating
	// the caller's slice elements (work on our own copy).
	templates := make([]config.MessageTemplate, len(settings.MessageTemplates))
	seenNames := make(map[string]struct{}, len(settings.MessageTemplates))
	for i, tmpl := range settings.MessageTemplates {
		name := strings.TrimSpace(tmpl.Name)
		msg := strings.TrimSpace(tmpl.Message)
		if name == "" {
			return fmt.Errorf("message_templates[%d]: name is required", i)
		}
		if msg == "" {
			return fmt.Errorf("message_templates[%d]: message is required", i)
		}
		if len([]rune(name)) > maxTemplateNameLen {
			return fmt.Errorf("message_templates[%d]: name exceeds maximum length of %d characters", i, maxTemplateNameLen)
		}
		if len([]rune(msg)) > maxTemplateMessageLen {
			return fmt.Errorf("message_templates[%d]: message exceeds maximum length of %d characters", i, maxTemplateMessageLen)
		}
		if _, exists := seenNames[name]; exists {
			return fmt.Errorf("message_templates[%d]: duplicate template name %q", i, name)
		}
		seenNames[name] = struct{}{}
		templates[i] = config.MessageTemplate{Name: name, Message: msg}
	}
	settings.MessageTemplates = templates

	event, err := a.configState.Update(func(cfg *config.Config) {
		cfg.TaskScheduler = &settings
	})
	if err != nil {
		return err
	}
	a.emitConfigUpdatedEvent(event)
	return nil
}
