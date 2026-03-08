package dreammaker

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
			name:    "direct dm-langserver command",
			command: "dm-langserver",
			want:    true,
		},
		{
			name:    "dreammaker executable path",
			command: `C:\tools\SpacemanDMM\dm-langserver.exe`,
			want:    true,
		},
		{
			name:    "cargo launch with dm-langserver bin",
			command: "cargo",
			args:    []string{"run", "--release", "--bin", "dm-langserver"},
			want:    true,
		},
		{
			name:    "arg contains spacemandmm path",
			command: "wrapper",
			args:    []string{`C:\src\SpacemanDMM\target\release\dm-langserver.exe`},
			want:    true,
		},
		{
			name:    "non dreammaker command",
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
	got := describeCapabilityCommand("dreammaker.reparse", "DreamMaker")
	if !strings.Contains(got, "DreamMaker") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
