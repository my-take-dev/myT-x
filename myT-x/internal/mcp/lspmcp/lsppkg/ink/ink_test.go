package ink

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
			name:    "direct ink language server command",
			command: "ink-language-server",
			want:    true,
		},
		{
			name:    "ink lsp server executable path",
			command: `C:\tools\ink\ink-lsp-server.exe`,
			want:    true,
		},
		{
			name:    "cargo run with ink lsp package",
			command: "cargo",
			args:    []string{"run", "--package", "ink-lsp-server", "--", "--stdio"},
			want:    true,
		},
		{
			name:    "arg contains ink analyzer path",
			command: "wrapper",
			args:    []string{`/opt/ink-analyzer/ink-analyzer`},
			want:    true,
		},
		{
			name:    "cargo unrelated package should not match",
			command: "cargo",
			args:    []string{"run", "--package", "rust-analyzer"},
			want:    false,
		},
		{
			name:    "non ink command",
			command: "rust-analyzer",
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
	got := describeCapabilityCommand("ink.expandMacro", "ink!")
	if !strings.Contains(got, "ink!") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
