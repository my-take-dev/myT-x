package fuzion

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
			name:    "direct fuzion-lsp command",
			command: "fuzion-lsp",
			want:    true,
		},
		{
			name:    "fuzion language server executable path",
			command: `C:\tools\fuzion\fuzion-language-server.exe`,
			want:    true,
		},
		{
			name:    "java launch with fuzion-lsp jar",
			command: "java",
			args:    []string{"-jar", `C:\tools\fuzion-lsp\fuzion-lsp.jar`},
			want:    true,
		},
		{
			name:    "fuzion runner with lsp subcommand",
			command: "fuzion",
			args:    []string{"lsp", "--stdio"},
			want:    true,
		},
		{
			name:    "arg contains fzls path",
			command: "wrapper",
			args:    []string{`C:\tools\fzls\fzls.exe`},
			want:    true,
		},
		{
			name:    "non fuzion command",
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
	got := describeCapabilityCommand("fuzion.compileModule", "Fuzion")
	if !strings.Contains(got, "Fuzion") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
