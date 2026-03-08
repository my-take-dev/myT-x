package deno

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
			name:    "direct denols command",
			command: "denols",
			want:    true,
		},
		{
			name:    "deno lsp executable path",
			command: `C:\tools\deno\deno-lsp.exe`,
			want:    true,
		},
		{
			name:    "deno lsp subcommand",
			command: "deno",
			args:    []string{"lsp"},
			want:    true,
		},
		{
			name:    "deno lsp with extra args",
			command: "deno",
			args:    []string{"lsp", "--unstable"},
			want:    true,
		},
		{
			name:    "arg contains denols path",
			command: "wrapper",
			args:    []string{`C:\servers\denols.exe`},
			want:    true,
		},
		{
			name:    "deno run is not lsp",
			command: "deno",
			args:    []string{"run", "main.ts"},
			want:    false,
		},
		{
			name:    "non deno command",
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
	got := describeCapabilityCommand("deno.cache", "Deno")
	if !strings.Contains(got, "Deno") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
