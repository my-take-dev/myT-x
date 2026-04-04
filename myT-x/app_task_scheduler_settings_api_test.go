package main

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"myT-x/internal/config"
)

// NOTE: This file overrides the package-level function variable
// runtimeEventsEmitFn. Do not use t.Parallel() here.

// TestTaskSchedulerConfigFieldCount guards against field addition drift.
// When adding fields to TaskSchedulerConfig, update this test and add corresponding
// test cases to ensure the new fields are validated and tested properly.
func TestTaskSchedulerConfigFieldCount(t *testing.T) {
	if got := reflect.TypeFor[config.TaskSchedulerConfig]().NumField(); got != 4 {
		t.Fatalf("TaskSchedulerConfig field count = %d, want 4; update TestTaskSchedulerConfigFieldCount for new fields", got)
	}
	if got := reflect.TypeFor[config.MessageTemplate]().NumField(); got != 2 {
		t.Fatalf("MessageTemplate field count = %d, want 2; update TestTaskSchedulerConfigFieldCount for new fields", got)
	}
}

// newConfigPathForTaskSchedulerTest is a thin wrapper over the shared newConfigPathForTest helper.
func newConfigPathForTaskSchedulerTest(t *testing.T, fileName string) string {
	t.Helper()
	return newConfigPathForTest(t, fileName)
}

func TestGetTaskSchedulerSettings_ReturnsZeroValueWhenTaskSchedulerIsNil(t *testing.T) {
	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.configState.Initialize(newConfigPathForTaskSchedulerTest(t, "config.yaml"), config.DefaultConfig())

	got := app.GetTaskSchedulerSettings()

	if got.PreExecResetDelay != 0 {
		t.Errorf("PreExecResetDelay = %d, want 0", got.PreExecResetDelay)
	}
	if got.PreExecIdleTimeout != 0 {
		t.Errorf("PreExecIdleTimeout = %d, want 0", got.PreExecIdleTimeout)
	}
	if got.PreExecTargetMode != "" {
		t.Errorf("PreExecTargetMode = %q, want empty", got.PreExecTargetMode)
	}
	if len(got.MessageTemplates) != 0 {
		t.Errorf("MessageTemplates length = %d, want 0", len(got.MessageTemplates))
	}
}

func TestGetTaskSchedulerSettings_ReturnsSavedSettings(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.configState.Initialize(newConfigPathForTaskSchedulerTest(t, "config.yaml"), config.DefaultConfig())

	runtimeEventsEmitFn = func(_ context.Context, _ string, _ ...any) {}

	settings := config.TaskSchedulerConfig{
		PreExecResetDelay:  30,
		PreExecIdleTimeout: 300,
		PreExecTargetMode:  "task_panes",
		MessageTemplates: []config.MessageTemplate{
			{Name: "greeting", Message: "Hello from task scheduler"},
		},
	}

	if err := app.SaveTaskSchedulerSettings(settings); err != nil {
		t.Fatalf("SaveTaskSchedulerSettings() error = %v", err)
	}

	got := app.GetTaskSchedulerSettings()

	if got.PreExecResetDelay != 30 {
		t.Errorf("PreExecResetDelay = %d, want 30", got.PreExecResetDelay)
	}
	if got.PreExecIdleTimeout != 300 {
		t.Errorf("PreExecIdleTimeout = %d, want 300", got.PreExecIdleTimeout)
	}
	if got.PreExecTargetMode != "task_panes" {
		t.Errorf("PreExecTargetMode = %q, want %q", got.PreExecTargetMode, "task_panes")
	}
	if len(got.MessageTemplates) != 1 {
		t.Errorf("MessageTemplates length = %d, want 1", len(got.MessageTemplates))
	}
	if got.MessageTemplates[0].Name != "greeting" {
		t.Errorf("MessageTemplates[0].Name = %q, want %q", got.MessageTemplates[0].Name, "greeting")
	}
	if got.MessageTemplates[0].Message != "Hello from task scheduler" {
		t.Errorf("MessageTemplates[0].Message = %q, want %q", got.MessageTemplates[0].Message, "Hello from task scheduler")
	}
}

func TestSaveTaskSchedulerSettings(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	tests := []struct {
		name    string
		input   config.TaskSchedulerConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid settings with all fields populated",
			input: config.TaskSchedulerConfig{
				PreExecResetDelay:  15,
				PreExecIdleTimeout: 50,
				PreExecTargetMode:  "task_panes",
				MessageTemplates: []config.MessageTemplate{
					{Name: "init", Message: "Initializing task"},
					{Name: "done", Message: "Task completed"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid settings with no templates",
			input: config.TaskSchedulerConfig{
				PreExecResetDelay:  0,
				PreExecIdleTimeout: 10,
				PreExecTargetMode:  "all_panes",
			},
			wantErr: false,
		},
		{
			name: "valid settings with empty target mode (optional)",
			input: config.TaskSchedulerConfig{
				PreExecResetDelay:  30,
				PreExecIdleTimeout: 100,
				PreExecTargetMode:  "",
			},
			wantErr: false,
		},
		{
			name: "valid settings with boundary delays (min)",
			input: config.TaskSchedulerConfig{
				PreExecResetDelay:  0,
				PreExecIdleTimeout: 10,
				PreExecTargetMode:  "task_panes",
			},
			wantErr: false,
		},
		{
			name: "valid settings with boundary delays (max)",
			input: config.TaskSchedulerConfig{
				PreExecResetDelay:  60,
				PreExecIdleTimeout: 600,
				PreExecTargetMode:  "all_panes",
			},
			wantErr: false,
		},
		{
			name: "invalid: PreExecResetDelay negative",
			input: config.TaskSchedulerConfig{
				PreExecResetDelay:  -1,
				PreExecIdleTimeout: 50,
				PreExecTargetMode:  "task_panes",
			},
			wantErr: true,
			errMsg:  "pre_exec_reset_delay_s must be between 0 and 60",
		},
		{
			name: "invalid: PreExecResetDelay exceeds max",
			input: config.TaskSchedulerConfig{
				PreExecResetDelay:  61,
				PreExecIdleTimeout: 50,
				PreExecTargetMode:  "task_panes",
			},
			wantErr: true,
			errMsg:  "pre_exec_reset_delay_s must be between 0 and 60",
		},
		{
			name: "invalid: PreExecIdleTimeout below min",
			input: config.TaskSchedulerConfig{
				PreExecResetDelay:  30,
				PreExecIdleTimeout: 9,
				PreExecTargetMode:  "task_panes",
			},
			wantErr: true,
			errMsg:  "pre_exec_idle_timeout_s must be between 10 and 600",
		},
		{
			name: "invalid: PreExecIdleTimeout exceeds max",
			input: config.TaskSchedulerConfig{
				PreExecResetDelay:  30,
				PreExecIdleTimeout: 601,
				PreExecTargetMode:  "task_panes",
			},
			wantErr: true,
			errMsg:  "pre_exec_idle_timeout_s must be between 10 and 600",
		},
		{
			name: "invalid: PreExecTargetMode invalid value",
			input: config.TaskSchedulerConfig{
				PreExecResetDelay:  30,
				PreExecIdleTimeout: 50,
				PreExecTargetMode:  "invalid_mode",
			},
			wantErr: true,
			errMsg:  "pre_exec_target_mode must be 'task_panes' or 'all_panes'",
		},
		{
			name: "invalid: template name empty",
			input: config.TaskSchedulerConfig{
				PreExecResetDelay:  30,
				PreExecIdleTimeout: 50,
				PreExecTargetMode:  "task_panes",
				MessageTemplates: []config.MessageTemplate{
					{Name: "", Message: "No name template"},
				},
			},
			wantErr: true,
			errMsg:  "message_templates[0]: name is required",
		},
		{
			name: "invalid: template message empty",
			input: config.TaskSchedulerConfig{
				PreExecResetDelay:  30,
				PreExecIdleTimeout: 50,
				PreExecTargetMode:  "task_panes",
				MessageTemplates: []config.MessageTemplate{
					{Name: "nomsg", Message: ""},
				},
			},
			wantErr: true,
			errMsg:  "message_templates[0]: message is required",
		},
		{
			name: "invalid: multiple templates with first one invalid (empty name)",
			input: config.TaskSchedulerConfig{
				PreExecResetDelay:  30,
				PreExecIdleTimeout: 50,
				PreExecTargetMode:  "task_panes",
				MessageTemplates: []config.MessageTemplate{
					{Name: "", Message: "First"},
					{Name: "second", Message: "Second"},
				},
			},
			wantErr: true,
			errMsg:  "message_templates[0]: name is required",
		},
		{
			name: "invalid: multiple templates with second one invalid (empty message)",
			input: config.TaskSchedulerConfig{
				PreExecResetDelay:  30,
				PreExecIdleTimeout: 50,
				PreExecTargetMode:  "task_panes",
				MessageTemplates: []config.MessageTemplate{
					{Name: "first", Message: "First"},
					{Name: "second", Message: ""},
				},
			},
			wantErr: true,
			errMsg:  "message_templates[1]: message is required",
		},
		{
			name: "valid settings with template whitespace gets trimmed",
			input: config.TaskSchedulerConfig{
				PreExecResetDelay:  30,
				PreExecIdleTimeout: 50,
				PreExecTargetMode:  "task_panes",
				MessageTemplates: []config.MessageTemplate{
					{Name: "  trimmed  ", Message: "  msg  "},
				},
			},
			wantErr: false,
		},
		{
			name: "valid settings with target mode whitespace gets trimmed",
			input: config.TaskSchedulerConfig{
				PreExecResetDelay:  30,
				PreExecIdleTimeout: 50,
				PreExecTargetMode:  "  task_panes  ",
			},
			wantErr: false,
		},
		{
			name: "invalid: too many templates exceeds limit",
			input: func() config.TaskSchedulerConfig {
				templates := make([]config.MessageTemplate, maxMessageTemplates+1)
				for i := range templates {
					templates[i] = config.MessageTemplate{
						Name:    "tmpl-" + strings.Repeat("x", 5),
						Message: "msg",
					}
				}
				return config.TaskSchedulerConfig{
					PreExecResetDelay:  10,
					PreExecIdleTimeout: 50,
					PreExecTargetMode:  "task_panes",
					MessageTemplates:   templates,
				}
			}(),
			wantErr: true,
			errMsg:  "message_templates: maximum 50 templates allowed",
		},
		{
			name: "invalid: template name exceeds max length",
			input: config.TaskSchedulerConfig{
				PreExecResetDelay:  10,
				PreExecIdleTimeout: 50,
				PreExecTargetMode:  "task_panes",
				MessageTemplates: []config.MessageTemplate{
					{Name: strings.Repeat("a", maxTemplateNameLen+1), Message: "msg"},
				},
			},
			wantErr: true,
			errMsg:  "message_templates[0]: name exceeds maximum length of 100 characters",
		},
		{
			name: "invalid: template message exceeds max length",
			input: config.TaskSchedulerConfig{
				PreExecResetDelay:  10,
				PreExecIdleTimeout: 50,
				PreExecTargetMode:  "task_panes",
				MessageTemplates: []config.MessageTemplate{
					{Name: "ok", Message: strings.Repeat("m", maxTemplateMessageLen+1)},
				},
			},
			wantErr: true,
			errMsg:  "message_templates[0]: message exceeds maximum length of 5000 characters",
		},
		{
			name: "invalid: duplicate template names",
			input: config.TaskSchedulerConfig{
				PreExecResetDelay:  10,
				PreExecIdleTimeout: 50,
				PreExecTargetMode:  "task_panes",
				MessageTemplates: []config.MessageTemplate{
					{Name: "dup", Message: "first"},
					{Name: "dup", Message: "second"},
				},
			},
			wantErr: true,
			errMsg:  `message_templates[1]: duplicate template name "dup"`,
		},
		{
			name: "invalid: duplicate template names after trim",
			input: config.TaskSchedulerConfig{
				PreExecResetDelay:  10,
				PreExecIdleTimeout: 50,
				PreExecTargetMode:  "task_panes",
				MessageTemplates: []config.MessageTemplate{
					{Name: "dup", Message: "first"},
					{Name: "  dup  ", Message: "second"},
				},
			},
			wantErr: true,
			errMsg:  `message_templates[1]: duplicate template name "dup"`,
		},
		{
			name: "valid: exactly at template count limit",
			input: func() config.TaskSchedulerConfig {
				templates := make([]config.MessageTemplate, maxMessageTemplates)
				for i := range templates {
					templates[i] = config.MessageTemplate{
						Name:    "tmpl-" + strings.Repeat("x", 3) + "-" + strings.Repeat("0", 2),
						Message: "msg",
					}
					// Ensure unique names.
					templates[i].Name = "tmpl-" + string(rune('A'+i/26)) + string(rune('a'+i%26))
				}
				return config.TaskSchedulerConfig{
					PreExecResetDelay:  10,
					PreExecIdleTimeout: 50,
					PreExecTargetMode:  "task_panes",
					MessageTemplates:   templates,
				}
			}(),
			wantErr: false,
		},
		{
			name: "valid: template name exactly at max length",
			input: config.TaskSchedulerConfig{
				PreExecResetDelay:  10,
				PreExecIdleTimeout: 50,
				PreExecTargetMode:  "task_panes",
				MessageTemplates: []config.MessageTemplate{
					{Name: strings.Repeat("a", maxTemplateNameLen), Message: "msg"},
				},
			},
			wantErr: false,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp()
			app.setRuntimeContext(context.Background())
			// Use numeric index for config file name to avoid Windows path issues
			// with special characters (colons, spaces) in test names.
			configFileName := fmt.Sprintf("config_%03d.yaml", i)
			app.configState.Initialize(newConfigPathForTaskSchedulerTest(t, configFileName), config.DefaultConfig())

			eventCount := 0
			runtimeEventsEmitFn = func(_ context.Context, _ string, _ ...any) {
				eventCount++
			}

			err := app.SaveTaskSchedulerSettings(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("SaveTaskSchedulerSettings() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil && err.Error() != tt.errMsg {
				t.Errorf("SaveTaskSchedulerSettings() error = %q, want %q", err.Error(), tt.errMsg)
			}

			if tt.wantErr {
				if eventCount != 0 {
					t.Errorf("SaveTaskSchedulerSettings() emitted %d events on error, want 0", eventCount)
				}
			} else {
				if eventCount != 1 {
					t.Errorf("SaveTaskSchedulerSettings() emitted %d events, want 1", eventCount)
				}
			}
		})
	}
}

func TestSaveTaskSchedulerSettingsEmitsUpdateEvent(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.configState.Initialize(newConfigPathForTaskSchedulerTest(t, "config.yaml"), config.DefaultConfig())

	eventCount := 0
	var eventName string
	var eventPayload config.UpdatedEvent
	runtimeEventsEmitFn = func(_ context.Context, name string, data ...any) {
		eventCount++
		eventName = name
		if len(data) == 0 {
			return
		}
		payload, ok := data[0].(config.UpdatedEvent)
		if ok {
			eventPayload = payload
		}
	}

	settings := config.TaskSchedulerConfig{
		PreExecResetDelay:  25,
		PreExecIdleTimeout: 75,
		PreExecTargetMode:  "all_panes",
		MessageTemplates: []config.MessageTemplate{
			{Name: "test", Message: "Test message"},
		},
	}

	if err := app.SaveTaskSchedulerSettings(settings); err != nil {
		t.Fatalf("SaveTaskSchedulerSettings() error = %v", err)
	}

	if eventCount != 1 {
		t.Fatalf("event count = %d, want 1", eventCount)
	}

	if eventName != "config:updated" {
		t.Fatalf("event name = %q, want %q", eventName, "config:updated")
	}

	if eventPayload.Config.TaskScheduler == nil {
		t.Fatal("event payload TaskScheduler is nil, want non-nil")
	}

	got := eventPayload.Config.TaskScheduler
	if got.PreExecResetDelay != 25 {
		t.Errorf("event TaskScheduler.PreExecResetDelay = %d, want 25", got.PreExecResetDelay)
	}
	if got.PreExecIdleTimeout != 75 {
		t.Errorf("event TaskScheduler.PreExecIdleTimeout = %d, want 75", got.PreExecIdleTimeout)
	}
	if got.PreExecTargetMode != "all_panes" {
		t.Errorf("event TaskScheduler.PreExecTargetMode = %q, want %q", got.PreExecTargetMode, "all_panes")
	}
	if len(got.MessageTemplates) != 1 {
		t.Errorf("event TaskScheduler.MessageTemplates length = %d, want 1", len(got.MessageTemplates))
	}

	if eventPayload.Version != 1 {
		t.Fatalf("event version = %d, want 1", eventPayload.Version)
	}

	if eventPayload.UpdatedAtUnixMilli <= 0 {
		t.Fatalf("event updated_at_unix_milli = %d, want > 0", eventPayload.UpdatedAtUnixMilli)
	}
}

func TestSaveTaskSchedulerSettingsTrimsTemplateStrings(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.configState.Initialize(newConfigPathForTaskSchedulerTest(t, "config.yaml"), config.DefaultConfig())

	runtimeEventsEmitFn = func(_ context.Context, _ string, _ ...any) {}

	settings := config.TaskSchedulerConfig{
		PreExecResetDelay:  20,
		PreExecIdleTimeout: 50,
		PreExecTargetMode:  "  task_panes  ",
		MessageTemplates: []config.MessageTemplate{
			{Name: "  init  ", Message: "  Starting  "},
			{Name: "  done  ", Message: "  Finished  "},
		},
	}

	if err := app.SaveTaskSchedulerSettings(settings); err != nil {
		t.Fatalf("SaveTaskSchedulerSettings() error = %v", err)
	}

	got := app.GetTaskSchedulerSettings()

	if got.PreExecTargetMode != "task_panes" {
		t.Errorf("PreExecTargetMode = %q, want %q (trimmed)", got.PreExecTargetMode, "task_panes")
	}

	if got.MessageTemplates[0].Name != "init" {
		t.Errorf("MessageTemplates[0].Name = %q, want %q (trimmed)", got.MessageTemplates[0].Name, "init")
	}
	if got.MessageTemplates[0].Message != "Starting" {
		t.Errorf("MessageTemplates[0].Message = %q, want %q (trimmed)", got.MessageTemplates[0].Message, "Starting")
	}

	if got.MessageTemplates[1].Name != "done" {
		t.Errorf("MessageTemplates[1].Name = %q, want %q (trimmed)", got.MessageTemplates[1].Name, "done")
	}
	if got.MessageTemplates[1].Message != "Finished" {
		t.Errorf("MessageTemplates[1].Message = %q, want %q (trimmed)", got.MessageTemplates[1].Message, "Finished")
	}
}

// TestSaveTaskSchedulerSettingsInputMutationSafety verifies that the function
// does not mutate the caller's input slice elements (templates) while validating.
// Defensive copy in the implementation protects against mutation.
func TestSaveTaskSchedulerSettingsInputMutationSafety(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.configState.Initialize(newConfigPathForTaskSchedulerTest(t, "config.yaml"), config.DefaultConfig())

	runtimeEventsEmitFn = func(_ context.Context, _ string, _ ...any) {}

	originalSettings := config.TaskSchedulerConfig{
		PreExecResetDelay:  20,
		PreExecIdleTimeout: 50,
		PreExecTargetMode:  "  task_panes  ",
		MessageTemplates: []config.MessageTemplate{
			{Name: "  init  ", Message: "  Starting  "},
		},
	}

	// Make a copy of the original templates for comparison.
	originalTemplates := make([]config.MessageTemplate, len(originalSettings.MessageTemplates))
	copy(originalTemplates, originalSettings.MessageTemplates)

	if err := app.SaveTaskSchedulerSettings(originalSettings); err != nil {
		t.Fatalf("SaveTaskSchedulerSettings() error = %v", err)
	}

	// Verify caller's input was not mutated.
	if originalSettings.MessageTemplates[0].Name != originalTemplates[0].Name {
		t.Errorf("caller's template Name was mutated from %q to %q", originalTemplates[0].Name, originalSettings.MessageTemplates[0].Name)
	}
	if originalSettings.MessageTemplates[0].Message != originalTemplates[0].Message {
		t.Errorf("caller's template Message was mutated from %q to %q", originalTemplates[0].Message, originalSettings.MessageTemplates[0].Message)
	}

	// Verify the saved version has trimmed values.
	saved := app.GetTaskSchedulerSettings()
	if saved.MessageTemplates[0].Name != "init" {
		t.Errorf("saved template Name = %q, want %q (trimmed)", saved.MessageTemplates[0].Name, "init")
	}
	if saved.MessageTemplates[0].Message != "Starting" {
		t.Errorf("saved template Message = %q, want %q (trimmed)", saved.MessageTemplates[0].Message, "Starting")
	}
}

func TestSaveTaskSchedulerSettingsMultipleUpdatesIncrementVersion(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.configState.Initialize(newConfigPathForTaskSchedulerTest(t, "config.yaml"), config.DefaultConfig())

	var versions []uint64
	runtimeEventsEmitFn = func(_ context.Context, name string, data ...any) {
		if name != "config:updated" || len(data) == 0 {
			return
		}
		payload, ok := data[0].(config.UpdatedEvent)
		if ok {
			versions = append(versions, payload.Version)
		}
	}

	settings1 := config.TaskSchedulerConfig{
		PreExecResetDelay:  10,
		PreExecIdleTimeout: 20,
		PreExecTargetMode:  "task_panes",
	}

	settings2 := config.TaskSchedulerConfig{
		PreExecResetDelay:  30,
		PreExecIdleTimeout: 40,
		PreExecTargetMode:  "all_panes",
	}

	if err := app.SaveTaskSchedulerSettings(settings1); err != nil {
		t.Fatalf("SaveTaskSchedulerSettings(settings1) error = %v", err)
	}

	if err := app.SaveTaskSchedulerSettings(settings2); err != nil {
		t.Fatalf("SaveTaskSchedulerSettings(settings2) error = %v", err)
	}

	if len(versions) != 2 {
		t.Fatalf("version count = %d, want 2", len(versions))
	}

	if versions[0] != 1 || versions[1] != 2 {
		t.Fatalf("versions = %v, want [1 2]", versions)
	}
}

func TestSaveTaskSchedulerSettingsDoesNotEmitEventWhenContextIsNil(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	app := NewApp()
	// Don't call setRuntimeContext — leave ctx nil
	app.configState.Initialize(newConfigPathForTaskSchedulerTest(t, "config.yaml"), config.DefaultConfig())

	eventCount := 0
	runtimeEventsEmitFn = func(_ context.Context, _ string, _ ...any) {
		eventCount++
	}

	settings := config.TaskSchedulerConfig{
		PreExecResetDelay:  15,
		PreExecIdleTimeout: 50,
		PreExecTargetMode:  "task_panes",
	}

	if err := app.SaveTaskSchedulerSettings(settings); err != nil {
		t.Fatalf("SaveTaskSchedulerSettings() error = %v", err)
	}

	if eventCount != 0 {
		t.Fatalf("event count = %d, want 0 (no runtime context)", eventCount)
	}
}

func TestSaveTaskSchedulerSettingsPreservesOtherConfigFieldsOnPartialUpdate(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	app := NewApp()
	app.setRuntimeContext(context.Background())
	initial := config.DefaultConfig()
	initial.Shell = "cmd.exe"
	initial.Prefix = "Ctrl+a"
	app.configState.Initialize(newConfigPathForTaskSchedulerTest(t, "config.yaml"), initial)

	runtimeEventsEmitFn = func(_ context.Context, _ string, _ ...any) {}

	settings := config.TaskSchedulerConfig{
		PreExecResetDelay:  20,
		PreExecIdleTimeout: 60,
		PreExecTargetMode:  "task_panes",
	}

	if err := app.SaveTaskSchedulerSettings(settings); err != nil {
		t.Fatalf("SaveTaskSchedulerSettings() error = %v", err)
	}

	got := app.GetConfig()

	// Verify other config fields are preserved.
	if got.Shell != "cmd.exe" {
		t.Errorf("Shell after SaveTaskSchedulerSettings = %q, want %q", got.Shell, "cmd.exe")
	}
	if got.Prefix != "Ctrl+a" {
		t.Errorf("Prefix after SaveTaskSchedulerSettings = %q, want %q", got.Prefix, "Ctrl+a")
	}

	// Verify TaskScheduler was updated.
	if got.TaskScheduler == nil {
		t.Fatal("TaskScheduler is nil after SaveTaskSchedulerSettings")
	}
	if got.TaskScheduler.PreExecResetDelay != 20 {
		t.Errorf("TaskScheduler.PreExecResetDelay = %d, want 20", got.TaskScheduler.PreExecResetDelay)
	}
}

func TestSaveTaskSchedulerSettingsKeepsPreviousStateOnValidationError(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.configState.Initialize(newConfigPathForTaskSchedulerTest(t, "config.yaml"), config.DefaultConfig())

	runtimeEventsEmitFn = func(_ context.Context, _ string, _ ...any) {}

	// Save valid settings.
	valid := config.TaskSchedulerConfig{
		PreExecResetDelay:  15,
		PreExecIdleTimeout: 75,
		PreExecTargetMode:  "task_panes",
		MessageTemplates: []config.MessageTemplate{
			{Name: "valid", Message: "Valid message"},
		},
	}

	if err := app.SaveTaskSchedulerSettings(valid); err != nil {
		t.Fatalf("SaveTaskSchedulerSettings(valid) error = %v", err)
	}

	validSnapshot := app.GetTaskSchedulerSettings()

	// Attempt to save invalid settings.
	invalid := config.TaskSchedulerConfig{
		PreExecResetDelay:  100, // Out of range
		PreExecIdleTimeout: 75,
		PreExecTargetMode:  "task_panes",
	}

	if err := app.SaveTaskSchedulerSettings(invalid); err == nil {
		t.Fatal("SaveTaskSchedulerSettings(invalid) expected error, got nil")
	}

	// Verify previous state is preserved.
	afterFailure := app.GetTaskSchedulerSettings()

	if afterFailure.PreExecResetDelay != validSnapshot.PreExecResetDelay {
		t.Errorf("PreExecResetDelay after failed save = %d, want %d (previous value)", afterFailure.PreExecResetDelay, validSnapshot.PreExecResetDelay)
	}
	if afterFailure.PreExecIdleTimeout != validSnapshot.PreExecIdleTimeout {
		t.Errorf("PreExecIdleTimeout after failed save = %d, want %d (previous value)", afterFailure.PreExecIdleTimeout, validSnapshot.PreExecIdleTimeout)
	}
	if len(afterFailure.MessageTemplates) != 1 {
		t.Errorf("MessageTemplates length after failed save = %d, want 1", len(afterFailure.MessageTemplates))
	}
	if afterFailure.MessageTemplates[0].Name != "valid" {
		t.Errorf("MessageTemplates[0].Name after failed save = %q, want %q", afterFailure.MessageTemplates[0].Name, "valid")
	}
}

func TestSaveTaskSchedulerSettingsEmitEventPayloadIsClone(t *testing.T) {
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() {
		runtimeEventsEmitFn = origEmit
	})

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.configState.Initialize(newConfigPathForTaskSchedulerTest(t, "config.yaml"), config.DefaultConfig())

	var eventPayload config.UpdatedEvent
	runtimeEventsEmitFn = func(_ context.Context, name string, data ...any) {
		if name != "config:updated" || len(data) == 0 {
			return
		}
		payload, ok := data[0].(config.UpdatedEvent)
		if ok {
			eventPayload = payload
		}
	}

	settings := config.TaskSchedulerConfig{
		PreExecResetDelay:  20,
		PreExecIdleTimeout: 50,
		PreExecTargetMode:  "task_panes",
	}

	if err := app.SaveTaskSchedulerSettings(settings); err != nil {
		t.Fatalf("SaveTaskSchedulerSettings() error = %v", err)
	}

	// Mutate event payload.
	if eventPayload.Config.TaskScheduler != nil {
		eventPayload.Config.TaskScheduler.PreExecResetDelay = 999
	}

	// Verify app state was not mutated.
	appSnapshot := app.GetTaskSchedulerSettings()
	if appSnapshot.PreExecResetDelay != 20 {
		t.Errorf("PreExecResetDelay after mutating event = %d, want 20 (should not be mutated)", appSnapshot.PreExecResetDelay)
	}
}
