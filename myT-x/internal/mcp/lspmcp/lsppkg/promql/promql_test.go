package promql

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
			name:    "direct promql langserver command",
			command: "promql-langserver",
			want:    true,
		},
		{
			name:    "promql language server executable path",
			command: `C:\tools\promql\promql-language-server.exe`,
			want:    true,
		},
		{
			name:    "go run with promql langserver module path",
			command: "go",
			args:    []string{"run", "github.com/prometheus-community/promql-langserver/cmd/promql-langserver@latest"},
			want:    true,
		},
		{
			name:    "wrapper args contain promql langserver path",
			command: "wrapper",
			args:    []string{`/usr/local/bin/promql-langserver`},
			want:    true,
		},
		{
			name:    "go run with unrelated prometheus tool",
			command: "go",
			args:    []string{"run", "github.com/prometheus/prometheus/cmd/promtool@latest"},
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
	got := describeCapabilityCommand("promql.evaluate", "PromQL")
	if !strings.Contains(got, "PromQL") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
