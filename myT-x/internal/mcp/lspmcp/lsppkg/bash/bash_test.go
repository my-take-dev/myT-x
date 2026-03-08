package bash

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
			name:    "direct bash language server command",
			command: "bash-language-server",
			want:    true,
		},
		{
			name:    "bash executable path",
			command: `C:\tools\bash-language-server\bash-language-server.cmd`,
			want:    true,
		},
		{
			name:    "node launch with bash language server arg",
			command: "node",
			args:    []string{`C:\tools\bash-language-server\out\server.js`},
			want:    true,
		},
		{
			name:    "arg contains bash language server path",
			command: "wrapper",
			args:    []string{`C:\tools\bash-language-server\bash-language-server`},
			want:    true,
		},
		{
			name:    "non bash command",
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
	got := describeCapabilityCommand("bash.format", "Bash")
	if !strings.Contains(got, "Bash") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
