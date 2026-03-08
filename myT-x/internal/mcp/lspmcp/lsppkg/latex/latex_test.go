package latex

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
			name:    "direct texlab command from mcp list",
			command: "texlab",
			want:    true,
		},
		{
			name:    "texlab executable path",
			command: `C:\tools\texlab\texlab.exe`,
			want:    true,
		},
		{
			name:    "cargo run with texlab package",
			command: "cargo",
			args:    []string{"run", "--package", "texlab", "--", "--stdio"},
			want:    true,
		},
		{
			name:    "cargo run with texlab bin",
			command: "cargo",
			args:    []string{"run", "--bin", "texlab"},
			want:    true,
		},
		{
			name:    "wrapper args contain texlab path",
			command: "wrapper",
			args:    []string{"--server", `/usr/local/bin/texlab`},
			want:    true,
		},
		{
			name:    "cargo run without texlab should not match",
			command: "cargo",
			args:    []string{"run", "--bin", "rust-analyzer"},
			want:    false,
		},
		{
			name:    "unrelated command",
			command: "ltex-ls",
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
	got := describeCapabilityCommand("workspace/executeCommand", "LaTeX")
	if !strings.Contains(got, "LaTeX") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
