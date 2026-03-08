package groovy

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
			name:    "direct groovy language server command",
			command: "groovy-language-server",
			want:    true,
		},
		{
			name:    "java launch with groovy language server jar",
			command: "java",
			args:    []string{"-jar", `C:\tools\groovy-language-server\groovy-language-server-all.jar`},
			want:    true,
		},
		{
			name:    "node launch with groovy lint language server entrypoint",
			command: "node",
			args:    []string{`C:\repo\node_modules\vscode-groovy-lint\server\groovy-lint-language-server.js`, "--stdio"},
			want:    true,
		},
		{
			name:    "npx launch with npm groovy lint language server",
			command: "npx",
			args:    []string{"npm-groovy-lint-language-server", "--stdio"},
			want:    true,
		},
		{
			name:    "arg contains groovy language server path",
			command: "wrapper",
			args:    []string{`C:\tools\prominic\groovy-language-server\bin\groovy-language-server`},
			want:    true,
		},
		{
			name:    "non groovy command",
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
	got := describeCapabilityCommand("groovy.organizeImports", "Groovy")
	if !strings.Contains(got, "Groovy") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
