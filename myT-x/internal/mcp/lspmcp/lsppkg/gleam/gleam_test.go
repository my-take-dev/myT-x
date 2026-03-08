package gleam

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
			name:    "direct gleam lsp command",
			command: "gleam-lsp",
			want:    true,
		},
		{
			name:    "gleam cli with lsp subcommand",
			command: "gleam",
			args:    []string{"lsp"},
			want:    true,
		},
		{
			name:    "cargo launch with gleam lsp arguments",
			command: "cargo",
			args:    []string{"run", "--package", "gleam", "--", "lsp"},
			want:    true,
		},
		{
			name:    "arg contains gleam language server path",
			command: "wrapper",
			args:    []string{`C:\tools\gleam-language-server\gleam-language-server.exe`},
			want:    true,
		},
		{
			name:    "non gleam command",
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
	got := describeCapabilityCommand("gleam.build", "Gleam")
	if !strings.Contains(got, "Gleam") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
