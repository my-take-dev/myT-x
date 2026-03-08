package yaml

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
			name:    "direct yaml language server command",
			command: "yaml-language-server",
			want:    true,
		},
		{
			name:    "vscode yaml language server executable path",
			command: `C:\tools\vscode-yaml\bin\vscode-yaml-language-server.cmd`,
			want:    true,
		},
		{
			name:    "node launch with redhat yaml language server script",
			command: "node",
			args:    []string{`C:\tools\yaml-language-server\out\server.js`},
			want:    true,
		},
		{
			name:    "wrapper args contain yaml language service repo hint",
			command: "wrapper",
			args:    []string{"--source", "adamt-voss/vscode-yaml-languageservice"},
			want:    true,
		},
		{
			name:    "node launch without yaml reference",
			command: "node",
			args:    []string{`C:\tools\wxml-languageserver\lib\server.js`},
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
	got := describeCapabilityCommand("yaml.refreshSchemaStore", "YAML")
	if !strings.Contains(got, "YAML") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
