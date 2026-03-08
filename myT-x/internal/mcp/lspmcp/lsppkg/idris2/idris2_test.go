package idris2

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
			name:    "direct idris2 lsp command",
			command: "idris2-lsp",
			want:    true,
		},
		{
			name:    "idris2 lsp executable path",
			command: `C:\tools\idris2\idris2-lsp.exe`,
			want:    true,
		},
		{
			name:    "pack run idris2 lsp",
			command: "pack",
			args:    []string{"run", "idris2-lsp"},
			want:    true,
		},
		{
			name:    "idris2 launch with lsp client argument",
			command: "idris2",
			args:    []string{"--client", "lsp"},
			want:    true,
		},
		{
			name:    "idris2 launch with non lsp client should not match",
			command: "idris2",
			args:    []string{"--client", "repl"},
			want:    false,
		},
		{
			name:    "arg contains idris2 lsp module path",
			command: "wrapper",
			args:    []string{"github.com/idris-community/idris2-lsp/cmd/idris2-lsp"},
			want:    true,
		},
		{
			name:    "idris2 repl command should not match",
			command: "idris2",
			args:    []string{"Main.idr"},
			want:    false,
		},
		{
			name:    "non idris2 command",
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
	got := describeCapabilityCommand("idris2.makeCase", "Idris2")
	if !strings.Contains(got, "Idris2") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
