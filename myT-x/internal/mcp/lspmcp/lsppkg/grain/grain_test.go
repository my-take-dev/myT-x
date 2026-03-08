package grain

import (
	"strings"
	"testing"
)

func TestMatches(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{
			name:    "direct grain language server command",
			command: "grain-language-server",
			want:    true,
		},
		{
			name:    "grain executable with lsp subcommand",
			command: "grain",
			args:    []string{"lsp"},
			want:    true,
		},
		{
			name:    "node launch with grain language server package",
			command: "node",
			args:    []string{`C:\repo\node_modules\@grain\language-server\dist\server.js`, "--stdio"},
			want:    true,
		},
		{
			name:    "npx launch with grain lsp package",
			command: "npx",
			args:    []string{"grain-lsp", "--stdio"},
			want:    true,
		},
		{
			name:    "grain command without lsp subcommand",
			command: "grain",
			args:    []string{"fmt"},
			want:    false,
		},
		{
			name:    "non grain command",
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
	got := describeCapabilityCommand("grain.resolveImport", "Grain")
	if !strings.Contains(got, "Grain") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
