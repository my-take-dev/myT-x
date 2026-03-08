package typescriptls

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
			name:    "direct typescript language server command",
			command: "typescript-language-server",
			want:    true,
		},
		{
			name:    "node launch with typescript language server path",
			command: "node",
			args:    []string{`C:\tools\typescript-language-server\lib\cli.js`, "--stdio"},
			want:    true,
		},
		{
			name:    "npx launch with typescript language server package",
			command: "npx",
			args:    []string{"typescript-language-server", "--stdio"},
			want:    true,
		},
		{
			name:    "sourcegraph javascript typescript language server should not match",
			command: "javascript-typescript-language-server",
			want:    false,
		},
		{
			name:    "node launch with sourcegraph javascript typescript language server should not match",
			command: "node",
			args:    []string{"javascript-typescript-language-server"},
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
	got := describeCapabilityCommand("workspace/executeCommand", "TypeScript")
	if !strings.Contains(got, "TypeScript") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
