package trinosql

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
			name:    "direct trinols command",
			command: "trinols",
			want:    true,
		},
		{
			name:    "trino ls executable path",
			command: `C:\tools\trino\trino-ls.exe`,
			want:    true,
		},
		{
			name:    "go launch with trinols module path",
			command: "go",
			args:    []string{"run", "github.com/rocket-boosters/trinols"},
			want:    true,
		},
		{
			name:    "wrapper args contain trinols path",
			command: "wrapper",
			args:    []string{`/usr/local/bin/trinols`},
			want:    true,
		},
		{
			name:    "non trino sql language server command",
			command: "trino-cli",
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
	got := describeCapabilityCommand("trino.executeQueryPlan", "Trino SQL")
	if !strings.Contains(got, "Trino SQL") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
