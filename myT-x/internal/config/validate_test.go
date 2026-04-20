package config

import (
	"reflect"
	"strings"
	"testing"
)

func newValidConfigWithTaskScheduler() Config {
	return Config{
		Shell:             "powershell.exe",
		Prefix:            "Ctrl+b",
		GlobalHotkey:      "Ctrl+Shift+F12",
		ViewerSidebarMode: "overlay",
		Keys: map[string]string{
			"split-vertical":   "%",
			"split-horizontal": "\"",
			"toggle-zoom":      "z",
			"kill-pane":        "x",
			"detach-session":   "d",
		},
	}
}

func TestTaskSchedulerConfigFieldCountGuard(t *testing.T) {
	const expectedFieldCount = 4
	if got := reflect.TypeFor[TaskSchedulerConfig]().NumField(); got != expectedFieldCount {
		t.Fatalf("TaskSchedulerConfig field count = %d, want %d; update validation, payload builders, and this assertion", got, expectedFieldCount)
	}
}

func TestMessageTemplateFieldCountGuard(t *testing.T) {
	const expectedFieldCount = 2
	if got := reflect.TypeFor[MessageTemplate]().NumField(); got != expectedFieldCount {
		t.Fatalf("MessageTemplate field count = %d, want %d; update sanitization, payload builders, and this assertion", got, expectedFieldCount)
	}
}

func TestApplyDefaultsAndValidate_TaskSchedulerTemplateLengthSanitization(t *testing.T) {
	cfg := newValidConfigWithTaskScheduler()
	cfg.TaskScheduler = &TaskSchedulerConfig{
		MessageTemplates: []MessageTemplate{
			{Name: strings.Repeat("a", MaxTemplateNameLen+1), Message: "valid"},
			{Name: "valid-name", Message: strings.Repeat("b", MaxTemplateMessageLen+1)},
			{Name: "kept", Message: "valid"},
		},
	}

	if err := applyDefaultsAndValidate(&cfg); err != nil {
		t.Fatalf("applyDefaultsAndValidate: %v", err)
	}
	if cfg.TaskScheduler == nil {
		t.Fatal("TaskScheduler should be preserved")
	}
	if len(cfg.TaskScheduler.MessageTemplates) != 1 {
		t.Fatalf("len(MessageTemplates) = %d, want 1", len(cfg.TaskScheduler.MessageTemplates))
	}
	if got := cfg.TaskScheduler.MessageTemplates[0]; got.Name != "kept" || got.Message != "valid" {
		t.Fatalf("remaining template = %+v, want kept/valid", got)
	}
}

func TestApplyDefaultsAndValidate_TaskSchedulerAppliesDefaultsAndFiltersTemplates(t *testing.T) {
	cfg := newValidConfigWithTaskScheduler()
	cfg.TaskScheduler = &TaskSchedulerConfig{
		PreExecResetDelay:  61,
		PreExecIdleTimeout: 0,
		PreExecTargetMode:  "unknown_mode",
		MessageTemplates: []MessageTemplate{
			{Name: " ", Message: "ignored"},
			{Name: "blank-message", Message: " "},
			{Name: "duplicate", Message: "first"},
			{Name: "duplicate", Message: "second"},
			{Name: "kept-early", Message: "valid"},
			{Name: "kept-late", Message: "valid"},
		},
	}

	if err := applyDefaultsAndValidate(&cfg); err != nil {
		t.Fatalf("applyDefaultsAndValidate: %v", err)
	}
	if cfg.TaskScheduler == nil {
		t.Fatal("TaskScheduler should be preserved")
	}
	if got := cfg.TaskScheduler.PreExecResetDelay; got != 0 {
		t.Fatalf("PreExecResetDelay = %d, want 0", got)
	}
	if got := cfg.TaskScheduler.PreExecIdleTimeout; got != DefaultPreExecIdleTimeout {
		t.Fatalf("PreExecIdleTimeout = %d, want %d", got, DefaultPreExecIdleTimeout)
	}
	if got := cfg.TaskScheduler.PreExecTargetMode; got != TaskSchedulerPreExecTargetModeTaskPanes {
		t.Fatalf("PreExecTargetMode = %q, want %q", got, TaskSchedulerPreExecTargetModeTaskPanes)
	}

	gotTemplates := cfg.TaskScheduler.MessageTemplates
	if len(gotTemplates) != 3 {
		t.Fatalf("len(MessageTemplates) = %d, want 3", len(gotTemplates))
	}
	if gotTemplates[0].Name != "duplicate" || gotTemplates[0].Message != "first" {
		t.Fatalf("first template = %+v, want duplicate/first", gotTemplates[0])
	}
	if gotTemplates[1].Name != "kept-early" || gotTemplates[2].Name != "kept-late" {
		t.Fatalf("filtered templates = %+v, want kept entries to survive sanitization", gotTemplates)
	}
}

func TestApplyDefaultsAndValidate_TaskSchedulerTemplateLimitAppliesAfterDeduplication(t *testing.T) {
	cfg := newValidConfigWithTaskScheduler()
	templates := make([]MessageTemplate, 0, MaxMessageTemplates+1)
	templates = append(templates, MessageTemplate{Name: "duplicate", Message: "first"})
	for i := 1; i < MaxMessageTemplates; i++ {
		templates = append(templates, MessageTemplate{Name: "duplicate", Message: "ignored"})
	}
	templates = append(templates, MessageTemplate{Name: "kept-late", Message: "valid"})
	cfg.TaskScheduler = &TaskSchedulerConfig{
		MessageTemplates: templates,
	}

	if err := applyDefaultsAndValidate(&cfg); err != nil {
		t.Fatalf("applyDefaultsAndValidate: %v", err)
	}
	if cfg.TaskScheduler == nil {
		t.Fatal("TaskScheduler should be preserved")
	}
	if len(cfg.TaskScheduler.MessageTemplates) != 2 {
		t.Fatalf("len(MessageTemplates) = %d, want 2", len(cfg.TaskScheduler.MessageTemplates))
	}
	if cfg.TaskScheduler.MessageTemplates[1].Name != "kept-late" {
		t.Fatalf("late unique template = %+v, want kept-late", cfg.TaskScheduler.MessageTemplates[1])
	}
}

func TestApplyDefaultsAndValidate_TaskSchedulerTemplateLimitTruncatesValidTemplates(t *testing.T) {
	cfg := newValidConfigWithTaskScheduler()
	templates := make([]MessageTemplate, 0, MaxMessageTemplates+1)
	for i := range MaxMessageTemplates + 1 {
		templates = append(templates, MessageTemplate{
			Name:    strings.Repeat("t", 1) + string(rune('a'+(i%26))) + strings.Repeat("x", i/26),
			Message: "valid",
		})
	}
	cfg.TaskScheduler = &TaskSchedulerConfig{
		MessageTemplates: templates,
	}

	if err := applyDefaultsAndValidate(&cfg); err != nil {
		t.Fatalf("applyDefaultsAndValidate: %v", err)
	}
	if cfg.TaskScheduler == nil {
		t.Fatal("TaskScheduler should be preserved")
	}
	if len(cfg.TaskScheduler.MessageTemplates) != MaxMessageTemplates {
		t.Fatalf("len(MessageTemplates) = %d, want %d", len(cfg.TaskScheduler.MessageTemplates), MaxMessageTemplates)
	}
}

func TestApplyDefaultsAndValidate_CanonicalizesLegacyViewerShortcutAlias(t *testing.T) {
	cfg := newValidConfigWithTaskScheduler()
	cfg.ViewerShortcuts = map[string]string{
		"file-tree": "Shift+Ctrl+1",
	}

	if err := applyDefaultsAndValidate(&cfg); err != nil {
		t.Fatalf("applyDefaultsAndValidate: %v", err)
	}

	want := map[string]string{"file-view": "Ctrl+Shift+1"}
	if !reflect.DeepEqual(cfg.ViewerShortcuts, want) {
		t.Fatalf("ViewerShortcuts = %#v, want %#v", cfg.ViewerShortcuts, want)
	}
}

func TestApplyDefaultsAndValidate_DropsReservedViewerShortcut(t *testing.T) {
	cfg := newValidConfigWithTaskScheduler()
	cfg.ViewerShortcuts = map[string]string{
		"git-graph": "Ctrl+Shift+V",
	}

	if err := applyDefaultsAndValidate(&cfg); err != nil {
		t.Fatalf("applyDefaultsAndValidate: %v", err)
	}

	if cfg.ViewerShortcuts != nil {
		t.Fatalf("ViewerShortcuts = %#v, want nil", cfg.ViewerShortcuts)
	}
}

func TestApplyDefaultsAndValidate_DropsViewerShortcutConflictingWithGlobalHotkey(t *testing.T) {
	cfg := newValidConfigWithTaskScheduler()
	cfg.GlobalHotkey = "Ctrl+Shift+1"
	cfg.ViewerShortcuts = map[string]string{
		"git-graph": "Shift+Ctrl+1",
	}

	if err := applyDefaultsAndValidate(&cfg); err != nil {
		t.Fatalf("applyDefaultsAndValidate: %v", err)
	}

	if cfg.ViewerShortcuts != nil {
		t.Fatalf("ViewerShortcuts = %#v, want nil", cfg.ViewerShortcuts)
	}
}

func TestApplyDefaultsAndValidate_DropsViewerShortcutConflictingWithDefaultShortcut(t *testing.T) {
	cfg := newValidConfigWithTaskScheduler()
	cfg.ViewerShortcuts = map[string]string{
		"git-graph": "Ctrl+Shift+E",
	}

	if err := applyDefaultsAndValidate(&cfg); err != nil {
		t.Fatalf("applyDefaultsAndValidate: %v", err)
	}

	if cfg.ViewerShortcuts != nil {
		t.Fatalf("ViewerShortcuts = %#v, want nil", cfg.ViewerShortcuts)
	}
}

func TestApplyDefaultsAndValidate_ReservedGlobalHotkeyFallsBackWhenQuakeModeEnabled(t *testing.T) {
	cfg := newValidConfigWithTaskScheduler()
	cfg.QuakeMode = true
	cfg.GlobalHotkey = "Ctrl+Shift+V"

	if err := applyDefaultsAndValidate(&cfg); err != nil {
		t.Fatalf("applyDefaultsAndValidate: %v", err)
	}

	if cfg.GlobalHotkey != DefaultConfig().GlobalHotkey {
		t.Fatalf("GlobalHotkey = %q, want %q", cfg.GlobalHotkey, DefaultConfig().GlobalHotkey)
	}
}
