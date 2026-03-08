package glimmer

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
			name:    "direct glint language server command",
			command: "glint-language-server",
			want:    true,
		},
		{
			name:    "node launch with glint language server entrypoint",
			command: "node",
			args:    []string{`C:\repo\node_modules\@glint\language-server\dist\server.js`, "--stdio"},
			want:    true,
		},
		{
			name:    "npx launch with glint language server package",
			command: "npx",
			args:    []string{"@glint/language-server", "--stdio"},
			want:    true,
		},
		{
			name:    "glint cli with lsp subcommand",
			command: "glint",
			args:    []string{"lsp"},
			want:    true,
		},
		{
			name:    "non glimmer command",
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
	got := describeCapabilityCommand("glint.check", "Glimmer templates")
	if !strings.Contains(got, "Glimmer templates") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
