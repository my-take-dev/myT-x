package teal

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
			name:    "direct teal language server command",
			command: "teal-language-server",
			want:    true,
		},
		{
			name:    "teal language server executable path",
			command: `C:\tools\teal\teal-language-server.exe`,
			want:    true,
		},
		{
			name:    "lua launch with teal language server script",
			command: "lua",
			args:    []string{"/opt/teal-language-server/main.lua"},
			want:    true,
		},
		{
			name:    "tl cli in lsp mode",
			command: "tl",
			args:    []string{"lsp"},
			want:    true,
		},
		{
			name:    "wrapper command with teal language server module path arg",
			command: "wrapper",
			args:    []string{"github.com/teal-language/teal-language-server"},
			want:    true,
		},
		{
			name:    "tl check command should not match",
			command: "tl",
			args:    []string{"check", "main.tl"},
			want:    false,
		},
		{
			name:    "lua with unrelated script should not match",
			command: "lua",
			args:    []string{"script.lua"},
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
	got := describeCapabilityCommand("teal.workspace.refresh", "Teal")
	if !strings.Contains(got, "Teal") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
