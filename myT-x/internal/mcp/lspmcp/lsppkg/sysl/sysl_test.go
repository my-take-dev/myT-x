package sysl

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
			name:    "direct sysl lsp command",
			command: "sysl-lsp",
			want:    true,
		},
		{
			name:    "sysl language server executable path",
			command: `C:\tools\sysl\sysl-language-server.exe`,
			want:    true,
		},
		{
			name:    "sysl cli in lsp mode",
			command: "sysl",
			args:    []string{"lsp"},
			want:    true,
		},
		{
			name:    "go launch of sysl in lsp mode",
			command: "go",
			args:    []string{"run", "github.com/anz-bank/sysl/cmd/sysl", "language-server"},
			want:    true,
		},
		{
			name:    "wrapper command with syslls arg",
			command: "wrapper",
			args:    []string{"/opt/sysl/bin/syslls"},
			want:    true,
		},
		{
			name:    "sysl cli in non lsp mode",
			command: "sysl",
			args:    []string{"fmt", "model.sysl"},
			want:    false,
		},
		{
			name:    "go launch without lsp mode",
			command: "go",
			args:    []string{"run", "github.com/anz-bank/sysl/cmd/sysl", "validate"},
			want:    false,
		},
		{
			name:    "non sysl command",
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
	got := describeCapabilityCommand("sysl.workspace.reindex", "Sysl")
	if !strings.Contains(got, "Sysl") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
