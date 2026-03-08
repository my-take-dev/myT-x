package partiql

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
			name:    "direct aws lsp partiql command",
			command: "aws-lsp-partiql",
			want:    true,
		},
		{
			name:    "aws lsp partiql executable path",
			command: `C:\tools\aws-lsp-partiql\aws-lsp-partiql.cmd`,
			want:    true,
		},
		{
			name:    "node launch with aws partiql language server arg",
			command: "node",
			args:    []string{`C:\src\aws-partiql-language-server\dist\server.js`},
			want:    true,
		},
		{
			name:    "arg contains aws lsp partiql path",
			command: "wrapper",
			args:    []string{`C:\tools\aws-lsp-partiql\bin\aws-lsp-partiql`},
			want:    true,
		},
		{
			name:    "node launch with unrelated script",
			command: "node",
			args:    []string{`C:\tools\scripts\start.js`},
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
	got := describeCapabilityCommand("partiql.organizeImports", "PartiQL")
	if !strings.Contains(got, "PartiQL") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
