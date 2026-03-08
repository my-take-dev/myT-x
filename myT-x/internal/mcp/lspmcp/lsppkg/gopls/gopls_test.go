package gopls

import (
	"slices"
	"testing"

	"myT-x/internal/mcp/lspmcp/internal/lsp"
)

// TestMatches は Matches が gopls コマンド/引数を正しく判定することを検証する。
func TestMatches(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{
			name:    "direct gopls command",
			command: "gopls",
			want:    true,
		},
		{
			name:    "absolute gopls.exe path",
			command: `C:\Users\dev\go\bin\gopls.exe`,
			want:    true,
		},
		{
			name:    "go tool gopls",
			command: "go",
			args:    []string{"tool", "gopls"},
			want:    true,
		},
		{
			name:    "arg contains gopls path",
			command: "wrapper",
			args:    []string{`C:\Users\dev\go\bin\gopls.exe`},
			want:    true,
		},
		{
			name:    "non gopls command",
			command: "pyright-langserver",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Matches(tt.command, tt.args)
			if got != tt.want {
				t.Fatalf("Matches(%q, %v) = %v, want %v", tt.command, tt.args, got, tt.want)
			}
		})
	}
}

// TestStaticCommandCatalogCoverage は staticCommandCatalog が必須コマンドを網羅していることを検証する。
func TestStaticCommandCatalogCoverage(t *testing.T) {
	required := []string{
		"gopls.add_dependency",
		"gopls.add_import",
		"gopls.add_telemetry_counters",
		"gopls.add_test",
		"gopls.apply_fix",
		"gopls.assembly",
		"gopls.change_signature",
		"gopls.check_upgrades",
		"gopls.client_open_url",
		"gopls.diagnose_files",
		"gopls.doc",
		"gopls.edit_go_directive",
		"gopls.extract_to_new_file",
		"gopls.fetch_vulncheck_result",
		"gopls.free_symbols",
		"gopls.gc_details",
		"gopls.generate",
		"gopls.go_get_package",
		"gopls.list_imports",
		"gopls.list_known_packages",
		"gopls.lsp",
		"gopls.maybe_prompt_for_telemetry",
		"gopls.mem_stats",
		"gopls.modify_tags",
		"gopls.modules",
		"gopls.move_type",
		"gopls.package_symbols",
		"gopls.packages",
		"gopls.regenerate_cgo",
		"gopls.remove_dependency",
		"gopls.reset_go_mod_diagnostics",
		"gopls.run_go_work_command",
		"gopls.run_govulncheck",
		"gopls.run_tests",
		"gopls.scan_imports",
		"gopls.split_package",
		"gopls.start_debugging",
		"gopls.start_profile",
		"gopls.stop_profile",
		"gopls.tidy",
		"gopls.update_go_sum",
		"gopls.upgrade_dependency",
		"gopls.vendor",
		"gopls.views",
		"gopls.vulncheck",
		"gopls.workspace_stats",
	}

	for _, name := range required {
		spec, ok := staticCommandCatalog[name]
		if !ok {
			t.Fatalf("missing static command catalog entry: %s", name)
		}
		if spec.Description == "" {
			t.Fatalf("empty description for %s", name)
		}
	}
}

// TestBuildToolsWithNilClient は client が nil の場合に BuildTools が nil を返すことを検証する。
func TestBuildToolsWithNilClient(t *testing.T) {
	got := BuildTools(nil, "/tmp")
	if got != nil {
		t.Fatalf("BuildTools(nil, /tmp) = %v, want nil", got)
	}
}

// TestBuildTools は BuildTools が返すツールの構造を検証する。
func TestBuildTools(t *testing.T) {
	client := &lsp.Client{}
	tools := BuildTools(client, "/tmp")
	if len(tools) != 2 {
		t.Fatalf("BuildTools returned %d tools, want 2", len(tools))
	}
	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name] = true
		if tool.Handler == nil {
			t.Errorf("tool %s has nil Handler", tool.Name)
		}
		if tool.InputSchema == nil {
			t.Errorf("tool %s has nil InputSchema", tool.Name)
		}
		if schema := tool.InputSchema; schema != nil {
			if req, ok := schema["required"]; ok {
				if reqSlice, ok := req.([]string); ok && len(reqSlice) > 0 {
					// execute スキーマは command を required に持つ
					if tool.Name == "gopls_execute_command" && !contains(reqSlice, "command") {
						t.Errorf("gopls_execute_command schema required should include command, got %v", reqSlice)
					}
				}
			}
		}
	}
	if !names["gopls_list_extension_commands"] || !names["gopls_execute_command"] {
		t.Fatalf("BuildTools missing expected tool names, got %v", names)
	}
}

func contains(s []string, v string) bool {
	return slices.Contains(s, v)
}
