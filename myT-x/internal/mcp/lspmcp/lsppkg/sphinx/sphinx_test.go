package sphinx

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
			name:    "direct esbonio command",
			command: "esbonio",
			want:    true,
		},
		{
			name:    "esbonio executable path",
			command: `C:\tools\esbonio\esbonio.exe`,
			want:    true,
		},
		{
			name:    "python launch with esbonio module",
			command: "python",
			args:    []string{"-m", "esbonio.lsp"},
			want:    true,
		},
		{
			name:    "uv launch with esbonio",
			command: "uv",
			args:    []string{"run", "esbonio", "--stdio"},
			want:    true,
		},
		{
			name:    "non sphinx command",
			command: "sphinx-build",
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
	got := describeCapabilityCommand("sphinx.applyDocAction", "Sphinx")
	if !strings.Contains(got, "Sphinx") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
