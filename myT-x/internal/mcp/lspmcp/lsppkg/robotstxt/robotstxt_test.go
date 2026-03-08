package robotstxt

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
			name:    "direct robots txt language server command",
			command: "robots-txt-language-server",
			want:    true,
		},
		{
			name:    "direct robots dot txt language server command",
			command: "robots-dot-txt-language-server",
			want:    true,
		},
		{
			name:    "node launch with vscode robots dot txt support server",
			command: "node",
			args:    []string{`C:\tools\vscode-robots-dot-txt-support\server\out\server.js`},
			want:    true,
		},
		{
			name:    "npx launch with vscode robots dot txt support package",
			command: "npx",
			args:    []string{"vscode-robots-dot-txt-support", "--stdio"},
			want:    true,
		},
		{
			name:    "arg contains robots txt language server binary path",
			command: "wrapper",
			args:    []string{`/opt/robots-txt-language-server/bin/robots-txt-language-server`},
			want:    true,
		},
		{
			name:    "robots cli command does not match",
			command: "robots",
			want:    false,
		},
		{
			name:    "robots txt file path does not match server",
			command: "wrapper",
			args:    []string{`C:\workspace\site\robots.txt`},
			want:    false,
		},
		{
			name:    "node launch with unrelated script",
			command: "node",
			args:    []string{`C:\tools\scripts\main.js`},
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
	got := describeCapabilityCommand("robotstxt.validate", "robots.txt")
	if !strings.Contains(got, "robots.txt") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
