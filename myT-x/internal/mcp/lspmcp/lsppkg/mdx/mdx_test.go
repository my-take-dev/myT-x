package mdx

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
			name:    "direct mdx analyzer command from mcp list",
			command: "mdx-analyzer",
			want:    true,
		},
		{
			name:    "mdx language server executable path",
			command: `C:\tools\mdx-language-server\mdx-language-server.exe`,
			want:    true,
		},
		{
			name:    "node launch with mdx analyzer script path",
			command: "node",
			args:    []string{`/opt/mdx-js/mdx-analyzer/dist/server.js`},
			want:    true,
		},
		{
			name:    "npx launch with mdx analyzer package",
			command: "npx",
			args:    []string{"mdx-analyzer", "--stdio"},
			want:    true,
		},
		{
			name:    "wrapper args contain mdx language server path",
			command: "wrapper",
			args:    []string{"--server", `/usr/local/bin/mdx-language-server`},
			want:    true,
		},
		{
			name:    "node launch with unrelated analyzer should not match",
			command: "node",
			args:    []string{`/opt/remark-language-server/server.js`},
			want:    false,
		},
		{
			name:    "unrelated command",
			command: "marksman",
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
	got := describeCapabilityCommand("workspace/executeCommand", "MDX")
	if !strings.Contains(got, "MDX") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
