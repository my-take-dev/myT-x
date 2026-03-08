package copilot

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
			name:    "direct copilot language server command",
			command: "copilot-language-server",
			want:    true,
		},
		{
			name:    "node launch with scoped package path",
			command: "node",
			args:    []string{`C:\repo\node_modules\@github\copilot-language-server\dist\agent.js`},
			want:    true,
		},
		{
			name:    "npx launch with scoped package",
			command: "npx",
			args:    []string{"@github/copilot-language-server", "--stdio"},
			want:    true,
		},
		{
			name:    "pnpm exec with scoped package",
			command: "pnpm",
			args:    []string{"exec", "@github/copilot-language-server", "--stdio"},
			want:    true,
		},
		{
			name:    "non copilot language server command",
			command: "github-copilot-cli",
			want:    false,
		},
		{
			name:    "node with unrelated package",
			command: "node",
			args:    []string{`C:\repo\node_modules\@github\copilot-chat\dist\cli.js`},
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
	got := describeCapabilityCommand("copilot.acceptSuggestion", "GitHub Copilot")
	if !strings.Contains(got, "GitHub Copilot") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
