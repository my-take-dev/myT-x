package mcptool

import (
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

func makeStringAnySlice(count int, value string) []any {
	items := make([]any, 0, count)
	for range count {
		items = append(items, value)
	}
	return items
}
