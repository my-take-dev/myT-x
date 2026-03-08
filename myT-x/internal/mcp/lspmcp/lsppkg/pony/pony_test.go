package pony

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
			name:    "direct ponyls command",
			command: "PonyLS",
			want:    true,
		},
		{
			name:    "ponyls executable path",
			command: `C:\tools\ponyls\ponyls.exe`,
			want:    true,
		},
		{
			name:    "arg contains pony ls path",
			command: "wrapper",
			args:    []string{`/opt/pony-ls/bin/pony-ls`},
			want:    true,
		},
		{
			name:    "pony compiler command",
			command: "ponyc",
			want:    false,
		},
		{
			name:    "wrapper with unrelated pony tool",
			command: "wrapper",
			args:    []string{`/opt/pony-tools/ponydoc`},
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
	got := describeCapabilityCommand("pony.build", "Pony")
	if !strings.Contains(got, "Pony") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
