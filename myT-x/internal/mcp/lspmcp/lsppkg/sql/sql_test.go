package sql

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
			name:    "direct sqls command",
			command: "sqls",
			want:    true,
		},
		{
			name:    "sqls executable path",
			command: `C:\tools\sqls\sqls.exe`,
			want:    true,
		},
		{
			name:    "go launch with sqls module path",
			command: "go",
			args:    []string{"run", "github.com/lighttiger2505/sqls"},
			want:    true,
		},
		{
			name:    "arg contains sqls binary path",
			command: "wrapper",
			args:    []string{`/usr/local/bin/sqls`},
			want:    true,
		},
		{
			name:    "non sql language server command",
			command: "psql",
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
	got := describeCapabilityCommand("sql.applyMigration", "SQL")
	if !strings.Contains(got, "SQL") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
