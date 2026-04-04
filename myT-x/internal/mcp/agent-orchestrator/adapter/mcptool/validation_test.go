package mcptool

import (
	"reflect"
	"strings"
	"testing"
)

func TestRequiredAgentNameBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]any
		wantErr bool
	}{
		{"valid max length", map[string]any{"name": "a" + strings.Repeat("b", 63)}, false},
		{"too long", map[string]any{"name": strings.Repeat("a", 65)}, true},
		{"leading hyphen", map[string]any{"name": "-agent"}, true},
		{"leading dot", map[string]any{"name": ".agent"}, true},
		{"whitespace only", map[string]any{"name": "   "}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := requiredAgentName(tt.args, "name")
			if (err != nil) != tt.wantErr {
				t.Fatalf("requiredAgentName error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRequiredTaskIDBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]any
		wantErr bool
	}{
		{"valid", map[string]any{"task_id": "t-abc123"}, false},
		{"missing prefix", map[string]any{"task_id": "abc123"}, true},
		{"empty", map[string]any{"task_id": ""}, true},
		{"too long", map[string]any{"task_id": "t-" + strings.Repeat("a", 63)}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := requiredTaskID(tt.args, "task_id")
			if (err != nil) != tt.wantErr {
				t.Fatalf("requiredTaskID error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOptionalStringListBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]any
		wantErr bool
	}{
		{
			name:    "valid",
			args:    map[string]any{"skills": []any{"go", "testing"}},
			wantErr: false,
		},
		{
			name:    "too many items",
			args:    map[string]any{"skills": makeStringAnySlice(21, "go")},
			wantErr: true,
		},
		{
			name:    "empty item",
			args:    map[string]any{"skills": []any{"go", ""}},
			wantErr: true,
		},
		{
			name:    "item at max length",
			args:    map[string]any{"skills": []any{strings.Repeat("a", 100)}},
			wantErr: false,
		},
		{
			name:    "item too long",
			args:    map[string]any{"skills": []any{strings.Repeat("a", 101)}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := optionalStringList(tt.args, "skills", maxSkills, maxSkillLen)
			if (err != nil) != tt.wantErr {
				t.Fatalf("optionalStringList error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOptionalLinesBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		want    int
		wantErr bool
	}{
		{"default", nil, 50, false},
		{"zero", float64(0), 0, true},
		{"negative", float64(-1), 0, true},
		{"too large", float64(201), 0, true},
		{"float", float64(1.5), 0, true},
		{"valid", float64(20), 20, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]any{}
			if tt.value != nil {
				args["lines"] = tt.value
			}
			got, err := optionalLines(args, "lines", 50, maxCaptureLines)
			if (err != nil) != tt.wantErr {
				t.Fatalf("optionalLines error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Fatalf("optionalLines = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestOptionalStatusFilterRejectsInvalidValue(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		want    string
		wantErr bool
	}{
		{"default", nil, "all", false},
		{"valid", "failed", "failed", false},
		{"valid blocked", "blocked", "blocked", false},
		{"invalid", "running", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]any{}
			if tt.value != nil {
				args["status_filter"] = tt.value
			}
			got, err := optionalStatusFilter(args, "status_filter", "all")
			if (err != nil) != tt.wantErr {
				t.Fatalf("optionalStatusFilter error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Fatalf("optionalStatusFilter = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOptionalTaskIDList(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]any
		want    []string
		wantErr bool
	}{
		{name: "nil", args: map[string]any{}, want: nil},
		{name: "valid", args: map[string]any{"depends_on": []any{"t-a", "t-b"}}, want: []string{"t-a", "t-b"}},
		{name: "duplicate", args: map[string]any{"depends_on": []any{"t-a", "t-a"}}, wantErr: true},
		{name: "invalid id", args: map[string]any{"depends_on": []any{"bad"}}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := optionalTaskIDList(tt.args, "depends_on", maxDependsOnTasks)
			if (err != nil) != tt.wantErr {
				t.Fatalf("optionalTaskIDList error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("optionalTaskIDList = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRequiredBatchTasks(t *testing.T) {
	args := map[string]any{
		"tasks": []any{
			map[string]any{"agent_name": "worker-a", "message": "task 1"},
			map[string]any{"agent_name": "worker-b", "message": "task 2", "include_response_instructions": false},
		},
	}

	got, err := requiredBatchTasks(args, "tasks")
	if err != nil {
		t.Fatalf("requiredBatchTasks: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].AgentName != "worker-a" || got[0].Message != "task 1" || !got[0].IncludeResponseInstructions {
		t.Fatalf("unexpected first item: %+v", got[0])
	}
	if got[1].AgentName != "worker-b" || got[1].IncludeResponseInstructions {
		t.Fatalf("unexpected second item: %+v", got[1])
	}
}

func TestOptionalSkillListFormats(t *testing.T) {
	tests := []struct {
		name      string
		args      map[string]any
		wantLen   int
		wantErr   bool
		wantFirst string // expected first skill name
	}{
		{
			name:    "nil key",
			args:    map[string]any{},
			wantLen: 0,
		},
		{
			name:      "legacy string array",
			args:      map[string]any{"skills": []any{"Go", "API"}},
			wantLen:   2,
			wantFirst: "Go",
		},
		{
			name: "object array with description",
			args: map[string]any{"skills": []any{
				map[string]any{"name": "Go", "description": "backend"},
			}},
			wantLen:   1,
			wantFirst: "Go",
		},
		{
			name: "object array without description",
			args: map[string]any{"skills": []any{
				map[string]any{"name": "testing"},
			}},
			wantLen:   1,
			wantFirst: "testing",
		},
		{
			name:    "empty string name",
			args:    map[string]any{"skills": []any{""}},
			wantErr: true,
		},
		{
			name: "empty object name",
			args: map[string]any{"skills": []any{
				map[string]any{"name": ""},
			}},
			wantErr: true,
		},
		{
			name:    "too many items",
			args:    map[string]any{"skills": makeStringAnySlice(21, "x")},
			wantErr: true,
		},
		{
			name:    "name too long",
			args:    map[string]any{"skills": []any{strings.Repeat("a", 101)}},
			wantErr: true,
		},
		{
			name: "description too long",
			args: map[string]any{"skills": []any{
				map[string]any{"name": "Go", "description": strings.Repeat("あ", 401)},
			}},
			wantErr: true,
		},
		{
			name:    "invalid type in array",
			args:    map[string]any{"skills": []any{42}},
			wantErr: true,
		},
		{
			name:    "not an array",
			args:    map[string]any{"skills": "not-array"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := optionalSkillList(tt.args, "skills", maxSkills)
			if (err != nil) != tt.wantErr {
				t.Fatalf("optionalSkillList error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if len(got) != tt.wantLen {
				t.Fatalf("got %d skills, want %d", len(got), tt.wantLen)
			}
			if tt.wantFirst != "" && got[0].Name != tt.wantFirst {
				t.Fatalf("first skill name = %q, want %q", got[0].Name, tt.wantFirst)
			}
		})
	}
}

func TestOptionalSkillListPreservesDescription(t *testing.T) {
	args := map[string]any{"skills": []any{
		map[string]any{"name": "Go", "description": "Goコードのレビュー"},
	}}
	got, err := optionalSkillList(args, "skills", maxSkills)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[0].Description != "Goコードのレビュー" {
		t.Fatalf("description = %q, want %q", got[0].Description, "Goコードのレビュー")
	}
}

func TestRequiredSendMessageIDBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]any
		wantErr bool
	}{
		{"valid", map[string]any{"send_message_id": "m-abc123"}, false},
		{"missing prefix", map[string]any{"send_message_id": "abc123"}, true},
		{"wrong prefix", map[string]any{"send_message_id": "t-abc123"}, true},
		{"empty", map[string]any{"send_message_id": ""}, true},
		{"missing key", map[string]any{}, true},
		{"too long", map[string]any{"send_message_id": "m-" + strings.Repeat("a", 63)}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := requiredSendMessageID(tt.args, "send_message_id")
			if (err != nil) != tt.wantErr {
				t.Fatalf("requiredSendMessageID error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func makeStringAnySlice(count int, value string) []any {
	items := make([]any, 0, count)
	for range count {
		items = append(items, value)
	}
	return items
}

func TestOptionalPaneIDBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		setKey  bool // true の場合は value が nil でもキーを設定する
		want    string
		wantErr bool
	}{
		{"absent", nil, false, "", false},
		{"key present nil value", nil, true, "", false},
		{"empty string", "", true, "", false},
		{"whitespace only", "   ", true, "", false},
		{"valid", "%1", true, "%1", false},
		{"invalid format", "abc", true, "", true},
		{"virtual pane", "%virtual-sched", true, "%virtual-sched", false},
		{"non-string", 42, true, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]any{}
			if tt.setKey || tt.value != nil {
				args["pane_id"] = tt.value
			}
			got, err := optionalPaneID(args, "pane_id")
			if (err != nil) != tt.wantErr {
				t.Fatalf("optionalPaneID error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Fatalf("optionalPaneID = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOptionalIntBoundedBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		setKey  bool // true の場合は value が nil でもキーを設定する
		want    int
		wantErr bool
	}{
		{"absent", nil, false, 3000, false},
		{"key present nil value", nil, true, 3000, false},
		{"valid", float64(5000), true, 5000, false},
		{"at min", float64(1000), true, 1000, false},
		{"below min", float64(999), true, 0, true},
		{"at max", float64(30000), true, 30000, false},
		{"above max", float64(30001), true, 0, true},
		{"float", float64(1500.5), true, 0, true},
		{"non-number", "abc", true, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]any{}
			if tt.setKey || tt.value != nil {
				args["delay"] = tt.value
			}
			got, err := optionalIntBounded(args, "delay", 3000, 1000, 30000)
			if (err != nil) != tt.wantErr {
				t.Fatalf("optionalIntBounded error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Fatalf("optionalIntBounded = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestOptionalSplitDirectionBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		want    string
		wantErr bool
	}{
		{"absent", nil, "horizontal", false},
		{"horizontal", "horizontal", "horizontal", false},
		{"vertical", "vertical", "vertical", false},
		{"invalid", "diagonal", "", true},
		{"non-string", 42, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]any{}
			if tt.value != nil {
				args["direction"] = tt.value
			}
			got, err := optionalSplitDirection(args, "direction", "horizontal")
			if (err != nil) != tt.wantErr {
				t.Fatalf("optionalSplitDirection error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Fatalf("optionalSplitDirection = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRequiredStringBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]any
		maxLen  int
		wantErr bool
	}{
		{"valid", map[string]any{"title": "hello"}, 30, false},
		{"at max", map[string]any{"title": strings.Repeat("a", 30)}, 30, false},
		{"too long", map[string]any{"title": strings.Repeat("a", 31)}, 30, true},
		{"empty", map[string]any{"title": ""}, 30, true},
		{"missing", map[string]any{}, 30, true},
		{"whitespace", map[string]any{"title": "   "}, 30, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := requiredString(tt.args, "title", tt.maxLen)
			if (err != nil) != tt.wantErr {
				t.Fatalf("requiredString error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
