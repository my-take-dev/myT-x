package odin

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
			name:    "direct ols command",
			command: "ols",
			want:    true,
		},
		{
			name:    "ols executable path",
			command: `C:\tools\ols\ols.exe`,
			want:    true,
		},
		{
			name:    "odin launch with ols path",
			command: "odin",
			args:    []string{"lsp", `/opt/ols/ols`},
			want:    true,
		},
		{
			name:    "wrapper args contain danielgavin ols path",
			command: "wrapper",
			args:    []string{`--server`, `/src/github.com/DanielGavin/ols/ols`},
			want:    true,
		},
		{
			name:    "odin invocation without ols reference",
			command: "odin",
			args:    []string{"check", "."},
			want:    false,
		},
		{
			name:    "similar but unrelated command",
			command: "tools",
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
	got := describeCapabilityCommand("odin.workspaceSymbols", "Odin")
	if !strings.Contains(got, "Odin") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
