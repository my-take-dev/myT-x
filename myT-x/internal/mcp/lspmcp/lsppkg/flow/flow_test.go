package flow

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
			name:    "direct flow language server command",
			command: "flow-language-server",
			want:    true,
		},
		{
			name:    "flow language server executable path",
			command: `C:\tools\flow-language-server\flow-language-server.cmd`,
			want:    true,
		},
		{
			name:    "flow lsp subcommand",
			command: "flow",
			args:    []string{"lsp", "--from", "vscode"},
			want:    true,
		},
		{
			name:    "wrapper contains flow lsp invocation",
			command: "wrapper",
			args:    []string{"/usr/local/bin/flow", "lsp", "--from", "vscode"},
			want:    true,
		},
		{
			name:    "node launch with flow language server script",
			command: "node",
			args:    []string{`C:\tools\flow-language-server\bin\cli.js`},
			want:    true,
		},
		{
			name:    "flow command with non lsp subcommand",
			command: "flow",
			args:    []string{"status"},
			want:    false,
		},
		{
			name:    "node launch with unrelated script",
			command: "node",
			args:    []string{"server.js"},
			want:    false,
		},
		{
			name:    "unrelated command",
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

func TestDescribeCapabilityCommand(t *testing.T) {
	got := describeCapabilityCommand("flow.organizeImports", "JavaScript Flow")
	if !strings.Contains(got, "JavaScript Flow") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
