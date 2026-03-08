package lox

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
			name:    "direct loxcraft command from mcp list",
			command: "loxcraft",
			want:    true,
		},
		{
			name:    "loxcraft executable path",
			command: `C:\tools\loxcraft\loxcraft.exe`,
			want:    true,
		},
		{
			name:    "arg contains loxcraft path",
			command: "wrapper",
			args:    []string{`/opt/loxcraft/bin/loxcraft`},
			want:    true,
		},
		{
			name:    "unrelated lsp command",
			command: "lpc-language-server",
			want:    false,
		},
		{
			name:    "non lox command",
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
	got := describeCapabilityCommand("loxcraft.indexWorkspace", "Lox")
	if !strings.Contains(got, "Lox") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
