package tailwindcss

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
			name:    "direct tailwindcss language server command",
			command: "tailwindcss-language-server",
			want:    true,
		},
		{
			name:    "node launch with tailwindcss language server path",
			command: "node",
			args:    []string{`C:\tools\tailwindcss-language-server\dist\index.js`, "--stdio"},
			want:    true,
		},
		{
			name:    "npx launch with tailwind package reference",
			command: "npx",
			args:    []string{"@tailwindcss/language-server", "--stdio"},
			want:    true,
		},
		{
			name:    "tailwind css cli command should not match",
			command: "tailwindcss",
			args:    []string{"--watch"},
			want:    false,
		},
		{
			name:    "node without language server reference",
			command: "node",
			args:    []string{"build.js"},
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
	got := describeCapabilityCommand("workspace/executeCommand", "Tailwind CSS")
	if !strings.Contains(got, "Tailwind CSS") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
