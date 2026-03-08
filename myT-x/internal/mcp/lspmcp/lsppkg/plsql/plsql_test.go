package plsql

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
			name:    "direct plsql language server command from mcp list",
			command: "plsql-language-server",
			want:    true,
		},
		{
			name:    "plsql language server executable path",
			command: `C:\tools\plsql-language-server\plsql-language-server.cmd`,
			want:    true,
		},
		{
			name:    "java launch with plsql language server jar",
			command: "java",
			args:    []string{"-jar", "/opt/plsql-language-server/plsql-language-server.jar"},
			want:    true,
		},
		{
			name:    "wrapper command with plsql language server path arg",
			command: "wrapper",
			args:    []string{"/usr/local/bin/plsql-language-server"},
			want:    true,
		},
		{
			name:    "java launch with unrelated jar should not match",
			command: "java",
			args:    []string{"-jar", "/opt/sql-language-tools/sql-formatter.jar"},
			want:    false,
		},
		{
			name:    "plsql formatter should not match",
			command: "plsql-formatter",
			want:    false,
		},
		{
			name:    "oracle sqlplus should not match",
			command: "sqlplus",
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
	got := describeCapabilityCommand("plsql.index.rebuild", "PL/SQL")
	if !strings.Contains(got, "PL/SQL") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
