package toml

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
			name:    "direct taplo command",
			command: "taplo",
			want:    true,
		},
		{
			name:    "direct tombi ls command",
			command: "tombi-ls",
			want:    true,
		},
		{
			name:    "npx launch with taplo cli package",
			command: "npx",
			args:    []string{"@taplo/cli", "lsp", "stdio"},
			want:    true,
		},
		{
			name:    "wrapper args contain tombi binary path",
			command: "wrapper",
			args:    []string{`/usr/local/bin/tombi`},
			want:    true,
		},
		{
			name:    "non toml command",
			command: "toml-sort",
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
	got := describeCapabilityCommand("toml.fixDocument", "TOML")
	if !strings.Contains(got, "TOML") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
