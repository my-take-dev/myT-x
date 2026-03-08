package agda

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
			name:    "direct agda language server command",
			command: "agda-language-server",
			want:    true,
		},
		{
			name:    "agda language server executable path",
			command: `C:\tools\agda-language-server\agda-language-server.exe`,
			want:    true,
		},
		{
			name:    "arg contains agda language server path",
			command: "wrapper",
			args:    []string{`C:\tools\agda-language-server\agda-language-server`},
			want:    true,
		},
		{
			name:    "non agda command",
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
	got := describeCapabilityCommand("agda.refine", "Agda")
	if !strings.Contains(got, "Agda/agda-language-server") {
		t.Fatalf("expected Agda/agda-language-server context in description, got %q", got)
	}
	if !strings.Contains(got, "agda | agda-language-server") {
		t.Fatalf("expected mcp_list context in description, got %q", got)
	}
	if !strings.Contains(got, "agda.refine") {
		t.Fatalf("expected command name in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
	if !strings.HasPrefix(got, "when: ") || !strings.Contains(got, " args: ") || !strings.HasSuffix(got, " effect: exec.") {
		t.Fatalf("expected triad description format, got %q", got)
	}
}

func TestBuildToolsDescriptionsIncludeAgdaServerContext(t *testing.T) {
	tools := BuildTools(&lsp.Client{}, "")
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}

	for _, tool := range tools {
		if !strings.Contains(tool.Description, "agda-language-server") {
			t.Fatalf("expected agda-language-server in %s description, got %q", tool.Name, tool.Description)
		}
		if !strings.Contains(tool.Description, "agda | agda-language-server") {
			t.Fatalf("expected mcp_list context in %s description, got %q", tool.Name, tool.Description)
		}
		if !strings.HasPrefix(tool.Description, "when: ") || !strings.Contains(tool.Description, " args: ") || !strings.Contains(tool.Description, " effect: ") {
			t.Fatalf("expected triad format in %s description, got %q", tool.Name, tool.Description)
		}
	}
}
