package debian

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
			name:    "direct debputy lsp command",
			command: "debputy-lsp",
			want:    true,
		},
		{
			name:    "debputy language server executable path",
			command: `C:\tools\debputy\debputy-language-server.exe`,
			want:    true,
		},
		{
			name:    "debputy cli with lsp subcommand",
			command: "debputy",
			args:    []string{"lsp", "server"},
			want:    true,
		},
		{
			name:    "python launch with debputy module",
			command: "python3",
			args:    []string{"-m", "debputy.lsp"},
			want:    true,
		},
		{
			name:    "uvx launch with debputy",
			command: "uvx",
			args:    []string{"debputy", "lsp"},
			want:    true,
		},
		{
			name:    "debputy cli non lsp command",
			command: "debputy",
			args:    []string{"build"},
			want:    false,
		},
		{
			name:    "non debian command",
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
	got := describeCapabilityCommand("debputy.lsp.reload", "Debian Packaging files")
	if !strings.Contains(got, "Debian Packaging files") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
