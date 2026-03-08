package testingls

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
			name:    "direct testing language server command",
			command: "testing-language-server",
			want:    true,
		},
		{
			name:    "node launch with testing language server script",
			command: "node",
			args:    []string{`C:\tools\testing-language-server\dist\server.js`},
			want:    true,
		},
		{
			name:    "npx launch with testing language server package",
			command: "npx",
			args:    []string{"testing-language-server", "--stdio"},
			want:    true,
		},
		{
			name:    "non testing language server command",
			command: "typescript-language-server",
			want:    false,
		},
		{
			name:    "node with unrelated package",
			command: "node",
			args:    []string{`C:\tools\eslint-language-server\lib\cli.js`},
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
	got := describeCapabilityCommand("testingls.discover", "Testing")
	if !strings.Contains(got, "Testing") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
