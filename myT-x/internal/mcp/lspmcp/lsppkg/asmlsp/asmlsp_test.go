package asmlsp

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
			name:    "direct asm lsp command from mcp list",
			command: "asm-lsp",
			want:    true,
		},
		{
			name:    "asm lsp executable path",
			command: `C:\tools\asm-lsp\asm-lsp.exe`,
			want:    true,
		},
		{
			name:    "cargo run with asm lsp package",
			command: "cargo",
			args:    []string{"run", "--package", "asm-lsp", "--", "--stdio"},
			want:    true,
		},
		{
			name:    "cargo run with asm lsp bin",
			command: "cargo",
			args:    []string{"run", "--bin", "asm-lsp"},
			want:    true,
		},
		{
			name:    "wrapper args contain asm lsp path",
			command: "wrapper",
			args:    []string{"--server", `/usr/local/bin/asm-lsp`},
			want:    true,
		},
		{
			name:    "cargo run without asm lsp should not match",
			command: "cargo",
			args:    []string{"run", "--bin", "rust-analyzer"},
			want:    false,
		},
		{
			name:    "unrelated command",
			command: "nasm",
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
	got := describeCapabilityCommand("workspace/executeCommand", "NASM/GO/GAS Assembly")
	if !strings.Contains(got, "NASM/GO/GAS Assembly") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
