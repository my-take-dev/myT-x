package gluon

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
			name:    "direct gluon language server command",
			command: "gluon-language-server",
			want:    true,
		},
		{
			name:    "gluon cli with lsp subcommand",
			command: "gluon",
			args:    []string{"lsp", "--stdio"},
			want:    true,
		},
		{
			name:    "cargo launch with gluon language server manifest",
			command: "cargo",
			args:    []string{"run", "--manifest-path", `C:\src\gluon-language-server\Cargo.toml`},
			want:    true,
		},
		{
			name:    "arg contains gluon lsp path",
			command: "wrapper",
			args:    []string{`C:\tools\gluon-lsp\gluon-lsp.exe`},
			want:    true,
		},
		{
			name:    "non gluon command",
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
	got := describeCapabilityCommand("gluon.eval", "Gluon")
	if !strings.Contains(got, "Gluon") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
