package polymer

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
			name:    "direct polymer editor service command from mcp list",
			command: "polymer-editor-service",
			want:    true,
		},
		{
			name:    "polymer editor service executable path",
			command: `C:\tools\polymer-editor-service\polymer-editor-service.cmd`,
			want:    true,
		},
		{
			name:    "node launch with polymer editor service script",
			command: "node",
			args:    []string{`/opt/polymer-editor-service/lib/language-server.js`, "--stdio"},
			want:    true,
		},
		{
			name:    "npx launch with polymer editor service package",
			command: "npx",
			args:    []string{"@polymer/polymer-editor-service", "--stdio"},
			want:    true,
		},
		{
			name:    "arg contains polymer editor server path",
			command: "wrapper",
			args:    []string{`C:\servers\polymer-editor-server\dist\server.js`},
			want:    true,
		},
		{
			name:    "polymer cli should not match",
			command: "polymer",
			want:    false,
		},
		{
			name:    "node launch with polymer analyzer should not match",
			command: "node",
			args:    []string{`/opt/polymer-analyzer/dist/index.js`},
			want:    false,
		},
		{
			name:    "html language server should not match",
			command: "vscode-html-languageserver",
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
	got := describeCapabilityCommand("polymer.navigate.toComponent", "Polymer")
	if !strings.Contains(got, "Polymer") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
