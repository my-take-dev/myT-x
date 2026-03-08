package diagnosticls

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
			name:    "direct diagnostic-languageserver command",
			command: "diagnostic-languageserver",
			want:    true,
		},
		{
			name:    "node launch with diagnostic-languageserver script",
			command: "node",
			args:    []string{`C:\tools\diagnostic-languageserver\bin\diagnostic-languageserver`},
			want:    true,
		},
		{
			name:    "npx launch with package name",
			command: "npx",
			args:    []string{"diagnostic-languageserver", "--stdio"},
			want:    true,
		},
		{
			name:    "arg contains diagnostic-languageserver path",
			command: "wrapper",
			args:    []string{`/usr/local/bin/diagnostic-languageserver`},
			want:    true,
		},
		{
			name:    "non diagnostic language server command",
			command: "eslint-language-server",
			want:    false,
		},
		{
			name:    "node with unrelated package",
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
	got := describeCapabilityCommand("diagnosticls.validate", "Diagnostic LS")
	if !strings.Contains(got, "Diagnostic LS") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
