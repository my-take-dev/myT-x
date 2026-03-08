package shader

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
			name:    "direct shader language server command",
			command: "shader-language-server",
			want:    true,
		},
		{
			name:    "shader language server executable path",
			command: `C:\tools\shader\shader-language-server.exe`,
			want:    true,
		},
		{
			name:    "wrapper args include shader lsp path",
			command: "wrapper",
			args:    []string{`C:\tools\shader-lsp\shader-lsp.exe`},
			want:    true,
		},
		{
			name:    "node launch shader language server script",
			command: "node",
			args:    []string{`C:\tools\shader-language-server\dist\server.js`},
			want:    true,
		},
		{
			name:    "node launch unrelated script",
			command: "node",
			args:    []string{`C:\tools\eslint\bin\eslint.js`},
			want:    false,
		},
		{
			name:    "unrelated command",
			command: "clangd",
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
	got := describeCapabilityCommand("shader.applyFix", "Shader")
	if !strings.Contains(got, "Shader") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
