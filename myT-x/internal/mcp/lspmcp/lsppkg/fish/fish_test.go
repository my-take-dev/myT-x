package fish

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
			name:    "direct fish lsp command",
			command: "fish-lsp",
			want:    true,
		},
		{
			name:    "fish language server executable path",
			command: `C:\tools\fish\fish-language-server.exe`,
			want:    true,
		},
		{
			name:    "node launch with fish lsp entrypoint",
			command: "node",
			args:    []string{`C:\tools\fish-lsp\dist\server.js`},
			want:    true,
		},
		{
			name:    "npx launch with fish lsp package",
			command: "npx",
			args:    []string{"fish-lsp", "--stdio"},
			want:    true,
		},
		{
			name:    "arg contains fish lsp path",
			command: "wrapper",
			args:    []string{`C:\servers\fish-lsp.cmd`},
			want:    true,
		},
		{
			name:    "non fish command",
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
	got := describeCapabilityCommand("fish.formatDocument", "fish")
	if !strings.Contains(got, "fish") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
