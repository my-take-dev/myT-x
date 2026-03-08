package tagls

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
			name:    "direct tagls command",
			command: "tagls",
			want:    true,
		},
		{
			name:    "node launch with tagls script",
			command: "node",
			args:    []string{`C:\tools\tagls\dist\server.js`},
			want:    true,
		},
		{
			name:    "npx launch with tagls package",
			command: "npx",
			args:    []string{"tagls", "--stdio"},
			want:    true,
		},
		{
			name:    "arg contains tag language server binary",
			command: "wrapper",
			args:    []string{`/usr/local/bin/tag-language-server`},
			want:    true,
		},
		{
			name:    "non tagls command",
			command: "gopls",
			want:    false,
		},
		{
			name:    "node with unrelated server",
			command: "node",
			args:    []string{`C:\tools\typescript-language-server\lib\cli.js`},
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
	got := describeCapabilityCommand("tagls.rename", "tagls")
	if !strings.Contains(got, "tagls") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
