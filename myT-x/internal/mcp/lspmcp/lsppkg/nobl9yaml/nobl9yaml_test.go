package nobl9yaml

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
			name:    "direct nobl9 vscode command",
			command: "nobl9-vscode",
			want:    true,
		},
		{
			name:    "nobl9 language server executable path",
			command: `C:\tools\nobl9-vscode\nobl9-language-server.exe`,
			want:    true,
		},
		{
			name:    "node launch with nobl9 vscode server script",
			command: "node",
			args:    []string{`C:\tools\nobl9-vscode\dist\language-server\server.js`},
			want:    true,
		},
		{
			name:    "wrapper args contain nobl9 yaml language server path",
			command: "wrapper",
			args:    []string{`--server`, `/opt/nobl9/nobl9-yaml-language-server`},
			want:    true,
		},
		{
			name:    "node launch without nobl9 reference",
			command: "node",
			args:    []string{`C:\tools\yaml-language-server\out\server.js`},
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
	got := describeCapabilityCommand("nobl9yaml.validate", "Nobl9 YAML")
	if !strings.Contains(got, "Nobl9 YAML") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
