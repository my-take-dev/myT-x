package ada

import (
	"strings"
	"testing"

	"myT-x/internal/mcp/lspmcp/internal/lsp"
)

func TestMatches(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{
			name:    "direct ada language server command",
			command: "ada_language_server",
			want:    true,
		},
		{
			name:    "ada language server executable path",
			command: `C:\tools\als\ada_language_server.exe`,
			want:    true,
		},
		{
			name:    "arg contains ada language server path",
			command: "wrapper",
			args:    []string{`C:\tools\als\ada-language-server.exe`},
			want:    true,
		},
		{
			name:    "non ada command",
			command: "gopls",
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

func TestDescribeCapabilityCommand(t *testing.T) {
	got := describeCapabilityCommand("als.refactor", "Ada/SPARK")
	if !strings.Contains(got, "Ada/SPARK") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "ada_language_server") {
		t.Fatalf("expected ada_language_server context in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
	if !strings.Contains(got, "when:") || !strings.Contains(got, "args:") || !strings.Contains(got, "effect:") {
		t.Fatalf("expected triad format in description, got %q", got)
	}
}

func TestBuildToolsDescriptions(t *testing.T) {
	tools := BuildTools(&lsp.Client{}, "")
	if len(tools) != 2 {
		t.Fatalf("BuildTools() returned %d tools, want 2", len(tools))
	}

	for _, tool := range tools {
		if !strings.Contains(tool.Description, "Ada/SPARK") {
			t.Fatalf("expected Ada/SPARK context in %q description, got %q", tool.Name, tool.Description)
		}
		if !strings.Contains(tool.Description, "ada_language_server") {
			t.Fatalf("expected ada_language_server context in %q description, got %q", tool.Name, tool.Description)
		}
		if !strings.Contains(tool.Description, "when:") || !strings.Contains(tool.Description, "args:") || !strings.Contains(tool.Description, "effect:") {
			t.Fatalf("expected triad format in %q description, got %q", tool.Name, tool.Description)
		}
	}
}
