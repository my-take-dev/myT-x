package m68k

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
			name:    "direct m68k lsp command from mcp list",
			command: "m68k-lsp",
			want:    true,
		},
		{
			name:    "m68k lsp executable path",
			command: `C:\tools\m68k-lsp\m68k-lsp.exe`,
			want:    true,
		},
		{
			name:    "node launch with m68k lsp script path",
			command: "node",
			args:    []string{`/opt/grahambates/m68k-lsp/dist/server.js`},
			want:    true,
		},
		{
			name:    "npx launch with m68k lsp package",
			command: "npx",
			args:    []string{"m68k-lsp", "--stdio"},
			want:    true,
		},
		{
			name:    "wrapper args contain m68k lsp path",
			command: "wrapper",
			args:    []string{"--server", `/usr/local/bin/m68k-lsp`},
			want:    true,
		},
		{
			name:    "node launch with unrelated lsp should not match",
			command: "node",
			args:    []string{`/opt/mips-lsp/server.js`},
			want:    false,
		},
		{
			name:    "unrelated command",
			command: "asm-lsp",
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
	got := describeCapabilityCommand("workspace/executeCommand", "Motorola 68000 Assembly")
	if !strings.Contains(got, "Motorola 68000 Assembly") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
