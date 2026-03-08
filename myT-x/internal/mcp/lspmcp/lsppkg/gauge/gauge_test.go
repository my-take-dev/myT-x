package gauge

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
			name:    "direct gauge-lsp command",
			command: "gauge-lsp",
			want:    true,
		},
		{
			name:    "gauge language server executable path",
			command: `C:\tools\gauge\gauge-language-server.exe`,
			want:    true,
		},
		{
			name:    "gauge cli with lsp subcommand",
			command: "gauge",
			args:    []string{"lsp", "--stdio"},
			want:    true,
		},
		{
			name:    "go run gauge lsp",
			command: "go",
			args:    []string{"run", "github.com/getgauge/gauge", "--", "lsp"},
			want:    true,
		},
		{
			name:    "gauge cli non lsp command",
			command: "gauge",
			args:    []string{"run", "specs"},
			want:    false,
		},
		{
			name:    "go run gauge non lsp",
			command: "go",
			args:    []string{"run", "github.com/getgauge/gauge", "--", "init"},
			want:    false,
		},
		{
			name:    "non gauge command",
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
	got := describeCapabilityCommand("gauge.runScenarios", "Gauge")
	if !strings.Contains(got, "Gauge") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
