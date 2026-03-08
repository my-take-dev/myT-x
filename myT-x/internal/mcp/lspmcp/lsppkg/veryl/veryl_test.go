package veryl

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
			name:    "direct veryl ls command",
			command: "veryl-ls",
			want:    true,
		},
		{
			name:    "veryl language server executable path",
			command: `C:\tools\veryl\veryl-language-server.exe`,
			want:    true,
		},
		{
			name:    "veryl cli lsp mode",
			command: "veryl",
			args:    []string{"lsp"},
			want:    true,
		},
		{
			name:    "cargo launch with veryl lsp binary",
			command: "cargo",
			args:    []string{"run", "--bin", "veryl-ls"},
			want:    true,
		},
		{
			name:    "veryl cli non lsp mode",
			command: "veryl",
			args:    []string{"fmt"},
			want:    false,
		},
		{
			name:    "plain veryl command should not match",
			command: "veryl",
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
	got := describeCapabilityCommand("workspace/executeCommand", "Veryl")
	if !strings.Contains(got, "Veryl") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
