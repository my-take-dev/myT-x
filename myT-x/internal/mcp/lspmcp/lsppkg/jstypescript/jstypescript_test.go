package jstypescript

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
			name:    "direct javascript typescript stdio command",
			command: "javascript-typescript-stdio",
			want:    true,
		},
		{
			name:    "javascript typescript langserver executable path",
			command: `C:\tools\javascript-typescript-langserver\javascript-typescript-langserver.exe`,
			want:    true,
		},
		{
			name:    "node launch with sourcegraph javascript typescript langserver",
			command: "node",
			args:    []string{`C:\tools\sourcegraph\javascript-typescript-langserver\lib\cli.js`},
			want:    true,
		},
		{
			name:    "direct biome lsp command",
			command: "biome_lsp",
			want:    true,
		},
		{
			name:    "biome cli with lsp proxy subcommand",
			command: "biome",
			args:    []string{"lsp-proxy"},
			want:    true,
		},
		{
			name:    "wrapper contains biome lsp invocation",
			command: "wrapper",
			args:    []string{"/usr/local/bin/biome", "lsp-proxy"},
			want:    true,
		},
		{
			name:    "biome cli with non lsp subcommand",
			command: "biome",
			args:    []string{"check", "."},
			want:    false,
		},
		{
			name:    "unrelated typescript language server",
			command: "typescript-language-server",
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
	got := describeCapabilityCommand("workspace/executeCommand", "JavaScript-Typescript")
	if !strings.Contains(got, "JavaScript-Typescript") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
