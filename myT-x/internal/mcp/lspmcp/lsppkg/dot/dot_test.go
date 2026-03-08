package dot

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
			name:    "direct dot language server command",
			command: "dot-language-server",
			want:    true,
		},
		{
			name:    "dot language server executable path",
			command: `C:\tools\dot-language-server\dot-language-server.exe`,
			want:    true,
		},
		{
			name:    "node launch with dot language server entrypoint",
			command: "node",
			args:    []string{`C:\repo\node_modules\dot-language-server\dist\server.js`, "--stdio"},
			want:    true,
		},
		{
			name:    "npx launch with dot language server package",
			command: "npx",
			args:    []string{"dot-language-server", "--stdio"},
			want:    true,
		},
		{
			name:    "graphviz dot renderer command should not match",
			command: "dot",
			want:    false,
		},
		{
			name:    "non dot command",
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
	got := describeCapabilityCommand("dot.preview", "Graphviz/DOT")
	if !strings.Contains(got, "Graphviz/DOT") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
