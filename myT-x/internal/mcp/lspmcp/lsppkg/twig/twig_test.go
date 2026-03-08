package twig

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
			name:    "direct twig language server command",
			command: "twig-language-server",
			want:    true,
		},
		{
			name:    "node launch with twig language server path",
			command: "node",
			args:    []string{"/opt/twig-language-server/dist/server.js", "--stdio"},
			want:    true,
		},
		{
			name:    "npx launch with repository path reference",
			command: "npx",
			args:    []string{"star-0/twig-language-server", "--stdio"},
			want:    true,
		},
		{
			name:    "args contain twig lsp binary path",
			command: "wrapper",
			args:    []string{"/usr/local/bin/twig-lsp"},
			want:    true,
		},
		{
			name:    "node without twig reference",
			command: "node",
			args:    []string{"script.js"},
			want:    false,
		},
		{
			name:    "unrelated twig tool",
			command: "twigcs",
			want:    false,
		},
		{
			name:    "unrelated command",
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
	got := describeCapabilityCommand("workspace/executeCommand", "Twig")
	if !strings.Contains(got, "Twig") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
