package slint

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
			name:    "direct slint lsp command",
			command: "slint-lsp",
			want:    true,
		},
		{
			name:    "slint language server executable path",
			command: `C:\tools\slint\slint-language-server.exe`,
			want:    true,
		},
		{
			name:    "wrapper args include slint lsp path",
			command: "wrapper",
			args:    []string{`C:\tools\slint-lsp\slint-lsp.exe`},
			want:    true,
		},
		{
			name:    "cargo launch slint lsp",
			command: "cargo",
			args:    []string{"run", "--bin", "slint-lsp"},
			want:    true,
		},
		{
			name:    "cargo launch unrelated binary",
			command: "cargo",
			args:    []string{"run", "--bin", "slint-viewer"},
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
	got := describeCapabilityCommand("slint.applyWorkspaceEdit", "Slint")
	if !strings.Contains(got, "Slint") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
