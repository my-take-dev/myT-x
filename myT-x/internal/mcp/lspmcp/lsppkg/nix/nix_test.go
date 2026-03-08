package nix

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
			name:    "direct nil command",
			command: "nil",
			want:    true,
		},
		{
			name:    "direct nixd executable path",
			command: `C:\tools\nix\nixd.exe`,
			want:    true,
		},
		{
			name:    "nix run nil from nixpkgs",
			command: "nix",
			args:    []string{"run", "nixpkgs#nil"},
			want:    true,
		},
		{
			name:    "nix run nixd from github",
			command: "nix",
			args:    []string{"run", "github:nix-community/nixd"},
			want:    true,
		},
		{
			name:    "wrapper args contain nixd path",
			command: "wrapper",
			args:    []string{"--server", `/usr/local/bin/nixd`},
			want:    true,
		},
		{
			name:    "nix invocation without nil or nixd reference",
			command: "nix",
			args:    []string{"run", "nixpkgs#hello"},
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
	got := describeCapabilityCommand("nix.reloadWorkspace", "Nix")
	if !strings.Contains(got, "Nix") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
