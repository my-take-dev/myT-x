package tools

import (
	"encoding/json"
	"reflect"
	"testing"

	"myT-x/internal/mcp/lspmcp/internal/lsp"
)

func TestBuildRegistryIncludesExpandedStandardTools(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	registry := BuildRegistry(client, ".", "dummy-lsp", nil)

	tools := registry.List()
	names := make(map[string]struct{}, len(tools))
	for _, tool := range tools {
		names[tool.Name] = struct{}{}
	}

	expected := []string{
		"lsp_get_declarations",
		"lsp_get_type_definitions",
		"lsp_get_implementations",
		"lsp_get_workspace_diagnostics",
		"lsp_prepare_rename",
		"lsp_format_range",
		"lsp_format_on_type",
		"lsp_execute_command",
		"lsp_resolve_completion_item",
		"lsp_resolve_code_action",
		"lsp_resolve_workspace_symbol",
	}

	for _, name := range expected {
		if _, ok := names[name]; !ok {
			t.Fatalf("expected tool %q in registry", name)
		}
	}
}

// --- IMP-2: extractDiagnosticsFromPullReport ---

func TestExtractDiagnosticsFromPullReport(t *testing.T) {
	t.Parallel()

	diag1 := map[string]any{"range": map[string]any{"start": map[string]any{"line": float64(1)}, "end": map[string]any{"line": float64(1)}}}
	diag2 := map[string]any{"range": map[string]any{"start": map[string]any{"line": float64(5)}, "end": map[string]any{"line": float64(5)}}}

	tests := []struct {
		name   string
		report any
		want   []any
	}{
		{
			name:   "report is nil",
			report: nil,
			want:   nil,
		},
		{
			name:   "report is slice not map",
			report: []any{},
			want:   nil,
		},
		{
			name:   "report is string",
			report: "string",
			want:   nil,
		},
		{
			name:   "report is map with no items",
			report: map[string]any{},
			want:   nil,
		},
		{
			name: "report is map with empty items",
			report: map[string]any{
				"items": []any{},
			},
			want: []any{},
		},
		{
			name: "report is map with items",
			report: map[string]any{
				"items": []any{diag1},
			},
			want: []any{diag1},
		},
		{
			name: "report with relatedDocuments",
			report: map[string]any{
				"items": []any{diag1},
				"relatedDocuments": map[string]any{
					"file:///b.go": map[string]any{"items": []any{diag2}},
				},
			},
			want: []any{diag1, diag2},
		},
		{
			name: "relatedDocuments sorted by key",
			report: map[string]any{
				"relatedDocuments": map[string]any{
					"file:///z.go": map[string]any{"items": []any{diag2}},
					"file:///a.go": map[string]any{"items": []any{diag1}},
				},
			},
			want: []any{diag1, diag2},
		},
		{
			name: "relatedDocuments with non-map value skipped",
			report: map[string]any{
				"relatedDocuments": map[string]any{
					"file:///a.go": "not a map",
					"file:///b.go": map[string]any{"items": []any{diag1}},
				},
			},
			want: []any{diag1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractDiagnosticsFromPullReport(tt.report)
			if !diagnosticsEqual(got, tt.want) {
				t.Fatalf("extractDiagnosticsFromPullReport() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- IMP-2: extractDiagnosticsFromWorkspaceReport ---

func TestExtractDiagnosticsFromWorkspaceReport(t *testing.T) {
	t.Parallel()

	diag1 := map[string]any{"message": "err1"}
	diag2 := map[string]any{"message": "err2"}
	diag3 := map[string]any{"message": "err3"}

	tests := []struct {
		name   string
		report any
		want   []any
	}{
		{
			name:   "nil report",
			report: nil,
			want:   nil,
		},
		{
			name:   "non-map report",
			report: "string",
			want:   nil,
		},
		{
			name:   "empty map",
			report: map[string]any{},
			want:   nil,
		},
		{
			name:   "items is not array",
			report: map[string]any{"items": "not array"},
			want:   nil,
		},
		{
			name:   "items is empty array",
			report: map[string]any{"items": []any{}},
			want:   []any{},
		},
		{
			name: "single document with diagnostics",
			report: map[string]any{
				"items": []any{
					map[string]any{
						"items": []any{diag1, diag2},
					},
				},
			},
			want: []any{diag1, diag2},
		},
		{
			name: "multiple documents with diagnostics",
			report: map[string]any{
				"items": []any{
					map[string]any{"items": []any{diag1}},
					map[string]any{"items": []any{diag2}},
				},
			},
			want: []any{diag1, diag2},
		},
		{
			name: "document with relatedDocuments",
			report: map[string]any{
				"items": []any{
					map[string]any{
						"items": []any{diag1},
						"relatedDocuments": map[string]any{
							"file:///related.go": map[string]any{"items": []any{diag2}},
						},
					},
				},
			},
			want: []any{diag1, diag2},
		},
		{
			name: "relatedDocuments sorted by key",
			report: map[string]any{
				"items": []any{
					map[string]any{
						"relatedDocuments": map[string]any{
							"file:///z.go": map[string]any{"items": []any{diag3}},
							"file:///a.go": map[string]any{"items": []any{diag2}},
						},
					},
				},
			},
			want: []any{diag2, diag3},
		},
		{
			name: "non-map item in items is skipped",
			report: map[string]any{
				"items": []any{
					"not a map",
					map[string]any{"items": []any{diag1}},
				},
			},
			want: []any{diag1},
		},
		{
			name: "relatedDocuments with non-map doc is skipped",
			report: map[string]any{
				"items": []any{
					map[string]any{
						"relatedDocuments": map[string]any{
							"file:///a.go": "not a map",
							"file:///b.go": map[string]any{"items": []any{diag1}},
						},
					},
				},
			},
			want: []any{diag1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractDiagnosticsFromWorkspaceReport(tt.report)
			if !diagnosticsEqual(got, tt.want) {
				t.Fatalf("extractDiagnosticsFromWorkspaceReport() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- IMP-3: extractLocations / parseLocation ---

func TestExtractLocations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    []lsp.Location
		wantErr bool
	}{
		{
			name: "null response",
			raw:  "null",
			want: nil,
		},
		{
			name: "empty string",
			raw:  "",
			want: nil,
		},
		{
			name: "single Location object",
			raw:  `{"uri":"file:///a.go","range":{"start":{"line":1,"character":2},"end":{"line":1,"character":5}}}`,
			want: []lsp.Location{
				{URI: "file:///a.go", Range: lsp.Range{Start: lsp.Position{Line: 1, Character: 2}, End: lsp.Position{Line: 1, Character: 5}}},
			},
		},
		{
			name: "Location array",
			raw: `[
				{"uri":"file:///a.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":3}}},
				{"uri":"file:///b.go","range":{"start":{"line":10,"character":5},"end":{"line":10,"character":8}}}
			]`,
			want: []lsp.Location{
				{URI: "file:///a.go", Range: lsp.Range{Start: lsp.Position{Line: 0, Character: 0}, End: lsp.Position{Line: 0, Character: 3}}},
				{URI: "file:///b.go", Range: lsp.Range{Start: lsp.Position{Line: 10, Character: 5}, End: lsp.Position{Line: 10, Character: 8}}},
			},
		},
		{
			name: "LocationLink with targetSelectionRange",
			raw:  `[{"targetUri":"file:///c.go","targetRange":{"start":{"line":5,"character":0},"end":{"line":5,"character":10}},"targetSelectionRange":{"start":{"line":5,"character":2},"end":{"line":5,"character":8}}}]`,
			want: []lsp.Location{
				{URI: "file:///c.go", Range: lsp.Range{Start: lsp.Position{Line: 5, Character: 2}, End: lsp.Position{Line: 5, Character: 8}}},
			},
		},
		{
			name: "LocationLink without targetSelectionRange falls back to targetRange",
			raw:  `[{"targetUri":"file:///d.go","targetRange":{"start":{"line":3,"character":0},"end":{"line":3,"character":15}}}]`,
			want: []lsp.Location{
				{URI: "file:///d.go", Range: lsp.Range{Start: lsp.Position{Line: 3, Character: 0}, End: lsp.Position{Line: 3, Character: 15}}},
			},
		},
		{
			name:    "invalid JSON array",
			raw:     `[broken`,
			wantErr: true,
		},
		{
			name: "invalid JSON object treated as no locations",
			raw:  `{broken`,
			want: nil,
		},
		{
			name: "empty array",
			raw:  `[]`,
			want: []lsp.Location{},
		},
		{
			name: "array with unparseable item is skipped",
			raw:  `[{"unknown":"field"},{"uri":"file:///a.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}]`,
			want: []lsp.Location{
				{URI: "file:///a.go", Range: lsp.Range{Start: lsp.Position{Line: 0, Character: 0}, End: lsp.Position{Line: 0, Character: 1}}},
			},
		},
		{
			name: "single object with no URI returns empty",
			raw:  `{"unknown":"field"}`,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := extractLocations(json.RawMessage(tt.raw))
			if (err != nil) != tt.wantErr {
				t.Fatalf("extractLocations() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if !locationsEqual(got, tt.want) {
				t.Fatalf("extractLocations() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- IMP-4: resolveRangeEnd ---

func TestResolveRangeEnd(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		args          map[string]any
		line          int
		character     int
		wantEndLine   int
		wantEndChar   int
		wantErr       bool
		wantErrSubstr string
	}{
		{
			name:        "endLine and endCharacter omitted defaults to start",
			args:        map[string]any{},
			line:        5,
			character:   10,
			wantEndLine: 5,
			wantEndChar: 10,
		},
		{
			name:        "valid range endLine > line",
			args:        map[string]any{"endLine": float64(10), "endCharacter": float64(3)},
			line:        5,
			character:   0,
			wantEndLine: 9, // endLine is 1-based, converted to 0-based
			wantEndChar: 3,
		},
		{
			name:          "invalid range endLine < line",
			args:          map[string]any{"endLine": float64(3)},
			line:          5,
			character:     0,
			wantErr:       true,
			wantErrSubstr: "end before start",
		},
		{
			name:          "endLine == line and endCharacter < character",
			args:          map[string]any{"endLine": float64(6), "endCharacter": float64(2)},
			line:          5,
			character:     5,
			wantErr:       true,
			wantErrSubstr: "end before start",
		},
		{
			name:        "endLine == line and endCharacter == character (zero-width range)",
			args:        map[string]any{"endLine": float64(6), "endCharacter": float64(5)},
			line:        5,
			character:   5,
			wantEndLine: 5,
			wantEndChar: 5,
		},
		{
			name:        "only endLine specified",
			args:        map[string]any{"endLine": float64(8)},
			line:        3,
			character:   2,
			wantEndLine: 7,
			wantEndChar: 2,
		},
		{
			name:        "only endCharacter specified",
			args:        map[string]any{"endCharacter": float64(20)},
			line:        3,
			character:   2,
			wantEndLine: 3,
			wantEndChar: 20,
		},
		{
			name:          "endLine is not an integer",
			args:          map[string]any{"endLine": "abc"},
			line:          0,
			character:     0,
			wantErr:       true,
			wantErrSubstr: "integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			endLine, endChar, err := resolveRangeEnd(tt.args, tt.line, tt.character)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrSubstr)
				}
				if tt.wantErrSubstr != "" && !contains(err.Error(), tt.wantErrSubstr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErrSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if endLine != tt.wantEndLine || endChar != tt.wantEndChar {
				t.Fatalf("resolveRangeEnd() = (%d, %d), want (%d, %d)", endLine, endChar, tt.wantEndLine, tt.wantEndChar)
			}
		})
	}
}

// --- SUG-8: truncateCompletions ---

func TestTruncateCompletions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		value         any
		maxItems      int
		wantTotal     int
		wantTruncated bool
		wantLen       int
	}{
		{
			name:          "slice under limit",
			value:         []any{"a", "b", "c"},
			maxItems:      5,
			wantTotal:     3,
			wantTruncated: false,
			wantLen:       3,
		},
		{
			name:          "slice over limit",
			value:         []any{"a", "b", "c", "d", "e"},
			maxItems:      3,
			wantTotal:     5,
			wantTruncated: true,
			wantLen:       3,
		},
		{
			name:          "slice at limit",
			value:         []any{"a", "b", "c"},
			maxItems:      3,
			wantTotal:     3,
			wantTruncated: false,
			wantLen:       3,
		},
		{
			name: "map with items under limit",
			value: map[string]any{
				"isIncomplete": true,
				"items":        []any{"a", "b"},
			},
			maxItems:      5,
			wantTotal:     2,
			wantTruncated: false,
			wantLen:       -1, // map: don't check len
		},
		{
			name: "map with items over limit",
			value: map[string]any{
				"isIncomplete": true,
				"items":        []any{"a", "b", "c", "d", "e"},
			},
			maxItems:      2,
			wantTotal:     5,
			wantTruncated: true,
			wantLen:       -1,
		},
		{
			name: "map without items key",
			value: map[string]any{
				"isIncomplete": false,
			},
			maxItems:      10,
			wantTotal:     0,
			wantTruncated: false,
			wantLen:       -1,
		},
		{
			name:          "default type returns 0",
			value:         "string value",
			maxItems:      10,
			wantTotal:     0,
			wantTruncated: false,
			wantLen:       -1,
		},
		{
			name:          "nil value",
			value:         nil,
			maxItems:      10,
			wantTotal:     0,
			wantTruncated: false,
			wantLen:       -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, total, truncated := truncateCompletions(tt.value, tt.maxItems)
			if total != tt.wantTotal {
				t.Fatalf("total = %d, want %d", total, tt.wantTotal)
			}
			if truncated != tt.wantTruncated {
				t.Fatalf("truncated = %v, want %v", truncated, tt.wantTruncated)
			}
			// slice の場合、切り詰め後の長さを検証
			if tt.wantLen >= 0 {
				items, ok := result.([]any)
				if !ok {
					t.Fatalf("expected []any result, got %T", result)
				}
				if len(items) != tt.wantLen {
					t.Fatalf("result len = %d, want %d", len(items), tt.wantLen)
				}
			}
			// map で truncated の場合、items が maxItems に切り詰められていることを検証
			if tt.wantTruncated {
				if m, ok := result.(map[string]any); ok {
					items := m["items"].([]any)
					if len(items) != tt.maxItems {
						t.Fatalf("map items len = %d, want %d", len(items), tt.maxItems)
					}
				}
			}
		})
	}
}

// --- SUG-8: byteOffsetToUTF16 ---

func TestByteOffsetToUTF16(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		line   string
		offset int
		want   int
	}{
		{
			name:   "zero offset",
			line:   "hello",
			offset: 0,
			want:   0,
		},
		{
			name:   "negative offset",
			line:   "hello",
			offset: -1,
			want:   0,
		},
		{
			name:   "ASCII only",
			line:   "hello",
			offset: 3,
			want:   3,
		},
		{
			name:   "ASCII full length",
			line:   "hello",
			offset: 5,
			want:   5,
		},
		{
			name:   "offset beyond line length clamped",
			line:   "hi",
			offset: 100,
			want:   2,
		},
		{
			name:   "Japanese characters (3-byte UTF-8 each, 1 UTF-16 code unit each)",
			line:   "日本語",
			offset: 3, // after '日' (3 bytes)
			want:   1,
		},
		{
			name:   "Japanese two chars",
			line:   "日本語",
			offset: 6, // after '日本' (6 bytes)
			want:   2,
		},
		{
			name:   "mixed ASCII and Japanese",
			line:   "a日b",
			offset: 1, // after 'a'
			want:   1,
		},
		{
			name:   "mixed ASCII and Japanese after kanji",
			line:   "a日b",
			offset: 4, // after 'a' + '日' (1+3=4 bytes)
			want:   2,
		},
		{
			name:   "emoji surrogate pair (4-byte UTF-8, 2 UTF-16 code units)",
			line:   "a😀b",
			offset: 1, // after 'a'
			want:   1,
		},
		{
			name:   "after emoji surrogate pair",
			line:   "a😀b",
			offset: 5, // after 'a' + '😀' (1+4=5 bytes)
			want:   3, // 'a'=1 + '😀'=2 UTF-16 code units
		},
		{
			name:   "empty line",
			line:   "",
			offset: 0,
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := byteOffsetToUTF16(tt.line, tt.offset)
			if got != tt.want {
				t.Fatalf("byteOffsetToUTF16(%q, %d) = %d, want %d", tt.line, tt.offset, got, tt.want)
			}
		})
	}
}

// --- collectRelatedDiagnostics ---

func TestCollectRelatedDiagnostics(t *testing.T) {
	t.Parallel()

	diag1 := map[string]any{"message": "a"}
	diag2 := map[string]any{"message": "b"}

	tests := []struct {
		name    string
		related map[string]any
		want    []any
	}{
		{
			name:    "empty map",
			related: map[string]any{},
			want:    nil,
		},
		{
			name: "single document",
			related: map[string]any{
				"file:///a.go": map[string]any{"items": []any{diag1}},
			},
			want: []any{diag1},
		},
		{
			name: "sorted by key",
			related: map[string]any{
				"file:///z.go": map[string]any{"items": []any{diag2}},
				"file:///a.go": map[string]any{"items": []any{diag1}},
			},
			want: []any{diag1, diag2},
		},
		{
			name: "non-map values skipped",
			related: map[string]any{
				"file:///a.go": "invalid",
				"file:///b.go": map[string]any{"items": []any{diag1}},
			},
			want: []any{diag1},
		},
		{
			name: "doc without items key",
			related: map[string]any{
				"file:///a.go": map[string]any{"other": "field"},
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := collectRelatedDiagnostics(tt.related)
			if !diagnosticsEqual(got, tt.want) {
				t.Fatalf("collectRelatedDiagnostics() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- テストヘルパー ---

// diagnosticsEqual は nil と空スライスを同等として比較する。
func diagnosticsEqual(got, want []any) bool {
	if len(got) == 0 && len(want) == 0 {
		// 両方 nil または空の場合は同等
		if want == nil && got != nil && len(got) > 0 {
			return false
		}
		if want != nil && len(want) == 0 && got == nil {
			// want が []any{} で got が nil → 同等とみなす
			return true
		}
		return true
	}
	return reflect.DeepEqual(got, want)
}

func locationsEqual(got, want []lsp.Location) bool {
	if len(got) == 0 && len(want) == 0 {
		return true
	}
	return reflect.DeepEqual(got, want)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
