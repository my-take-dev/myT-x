package tads3

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
			name:    "direct tads3 language server command",
			command: "tads3-language-server",
			want:    true,
		},
		{
			name:    "t3ls executable path",
			command: `C:\tools\tads3\t3ls.exe`,
			want:    true,
		},
		{
			name:    "tads3 cli in lsp mode",
			command: "tads3",
			args:    []string{"lsp"},
			want:    true,
		},
		{
			name:    "wrapper command with tads3 tools language server path arg",
			command: "wrapper",
			args:    []string{"/opt/tads3tools/bin/tads3-language-server"},
			want:    true,
		},
		{
			name:    "t3make compile command should not match",
			command: "t3make",
			args:    []string{"game.t"},
			want:    false,
		},
		{
			name:    "tads3 runtime command should not match",
			command: "tads3",
			args:    []string{"run", "story.t3"},
			want:    false,
		},
		{
			name:    "non tads3 command",
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
	got := describeCapabilityCommand("tads3.workspace.rebuildIndex", "Tads3")
	if !strings.Contains(got, "Tads3") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
