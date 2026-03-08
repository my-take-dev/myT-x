package crystal

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
			name:    "direct crystalline command",
			command: "crystalline",
			want:    true,
		},
		{
			name:    "scry executable path",
			command: `C:\tools\scry\scry.exe`,
			want:    true,
		},
		{
			name:    "arg contains crystalline path",
			command: "wrapper",
			args:    []string{`C:\tools\crystalline\crystalline`},
			want:    true,
		},
		{
			name:    "non crystal command",
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
	got := describeCapabilityCommand("crystal.build", "Crystal")
	if !strings.Contains(got, "Crystal") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
