package vala

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
			name:    "direct vala language server command",
			command: "vala-language-server",
			want:    true,
		},
		{
			name:    "vala language server executable path",
			command: `C:\tools\vala-language-server\vala-language-server.exe`,
			want:    true,
		},
		{
			name:    "wrapper args contain vala lsp path",
			command: "wrapper",
			args:    []string{"/usr/local/bin/vala-ls"},
			want:    true,
		},
		{
			name:    "vala compiler should not match",
			command: "vala",
			want:    false,
		},
		{
			name:    "valac compiler should not match",
			command: "valac",
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
	got := describeCapabilityCommand("workspace/executeCommand", "Vala")
	if !strings.Contains(got, "Vala") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
