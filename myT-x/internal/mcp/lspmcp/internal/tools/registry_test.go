package tools

import (
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

// TestExtractDiagnosticsFromPullReport は extractDiagnosticsFromPullReport の pull diagnostics レスポンス解析を検証する。
func TestExtractDiagnosticsFromPullReport(t *testing.T) {
	t.Parallel()

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
				"items": []any{
					map[string]any{"range": map[string]any{"start": map[string]any{"line": float64(1)}, "end": map[string]any{"line": float64(1)}}},
				},
			},
			want: []any{
				map[string]any{"range": map[string]any{"start": map[string]any{"line": float64(1)}, "end": map[string]any{"line": float64(1)}}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractDiagnosticsFromPullReport(tt.report)
			// nil と []any{} は DeepEqual で異なるが、len 0 の場合は同等とみなす
			if tt.want != nil && len(tt.want) == 0 && (got == nil || len(got) == 0) {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("extractDiagnosticsFromPullReport() = %v, want %v", got, tt.want)
			}
		})
	}
}
