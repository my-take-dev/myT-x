package typst

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
			name:    "direct tinymist command",
			command: "tinymist",
			want:    true,
		},
		{
			name:    "direct typst lsp command",
			command: "typst-lsp",
			want:    true,
		},
		{
			name:    "cargo launch with typst lsp binary",
			command: "cargo",
			args:    []string{"run", "--bin", "typst-lsp"},
			want:    true,
		},
		{
			name:    "rustup launch with tinymist command",
			command: "rustup",
			args:    []string{"run", "stable", "tinymist", "--lsp"},
			want:    true,
		},
		{
			name:    "wrapper args contain tinymist binary path",
			command: "wrapper",
			args:    []string{"/usr/local/bin/tinymist"},
			want:    true,
		},
		{
			name:    "cargo without typst server reference",
			command: "cargo",
			args:    []string{"run", "--bin", "rust-analyzer"},
			want:    false,
		},
		{
			name:    "typst cli should not match",
			command: "typst",
			args:    []string{"watch", "main.typ"},
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
	got := describeCapabilityCommand("workspace/executeCommand", "Typst")
	if !strings.Contains(got, "Typst") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
