package lpc

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
			name:    "direct lpc-language-server command from mcp list",
			command: "lpc-language-server",
			want:    true,
		},
		{
			name:    "lpc-language-server executable path",
			command: `C:\tools\lpc-language-server\lpc-language-server.exe`,
			want:    true,
		},
		{
			name:    "node launch with lpc-language-server script path",
			command: "node",
			args:    []string{`/opt/lsp/lpc-language-server/dist/server.js`, "--stdio"},
			want:    true,
		},
		{
			name:    "arg contains lpc-language-server binary path",
			command: "wrapper",
			args:    []string{`/opt/lsp/lpc-language-server/bin/lpc-language-server`},
			want:    true,
		},
		{
			name:    "node launch with unrelated script should not match",
			command: "node",
			args:    []string{`/opt/lsp/some-other-server/dist/server.js`, "--stdio"},
			want:    false,
		},
		{
			name:    "non lpc command",
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
	got := describeCapabilityCommand("lpc.organizeImports", "LPC")
	if !strings.Contains(got, "LPC") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
