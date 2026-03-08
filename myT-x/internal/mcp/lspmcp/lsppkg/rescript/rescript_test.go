package rescript

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
			name:    "direct rescript language server command",
			command: "rescript-language-server",
			want:    true,
		},
		{
			name:    "direct rescript editor analysis command",
			command: "rescript-editor-analysis",
			want:    true,
		},
		{
			name:    "node launch with rescript vscode server script",
			command: "node",
			args:    []string{`C:\tools\rescript-vscode\server\out\server.js`},
			want:    true,
		},
		{
			name:    "npx launch with language server package",
			command: "npx",
			args:    []string{"@rescript/language-server", "--stdio"},
			want:    true,
		},
		{
			name:    "arg contains rescript language server binary path",
			command: "wrapper",
			args:    []string{`/opt/rescript-language-server/bin/rescript-language-server`},
			want:    true,
		},
		{
			name:    "rescript compiler command does not match",
			command: "rescript",
			args:    []string{"build"},
			want:    false,
		},
		{
			name:    "node launch with unrelated script",
			command: "node",
			args:    []string{`C:\tools\scripts\main.js`},
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
	got := describeCapabilityCommand("rescript.organizeImports", "ReScript")
	if !strings.Contains(got, "ReScript") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
