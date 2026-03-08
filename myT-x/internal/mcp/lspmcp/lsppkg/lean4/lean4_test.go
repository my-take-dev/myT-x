package lean4

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
			name:    "direct Lean4 command from mcp list",
			command: "Lean4",
			want:    true,
		},
		{
			name:    "lean4 executable path",
			command: `C:\tools\lean4\lean4.exe`,
			want:    true,
		},
		{
			name:    "arg contains lean4 server path",
			command: "wrapper",
			args:    []string{`/opt/lean4/bin/lean4-language-server`},
			want:    true,
		},
		{
			name:    "lean3 binary should not match lean4 package",
			command: "lean",
			want:    false,
		},
		{
			name:    "non lean command",
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
	got := describeCapabilityCommand("lean4.restartFile", "Lean4")
	if !strings.Contains(got, "Lean4") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
