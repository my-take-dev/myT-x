package codeql

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
			name:    "direct codeql language server command",
			command: "codeql-language-server",
			want:    true,
		},
		{
			name:    "codeql executable path",
			command: `C:\tools\codeql-language-server\codeql-language-server.exe`,
			want:    true,
		},
		{
			name:    "java launch with codeql lsp arg",
			command: "java",
			args:    []string{"-jar", `C:\tools\codeql-lsp\codeql-lsp.jar`},
			want:    true,
		},
		{
			name:    "arg contains codeql lsp path",
			command: "wrapper",
			args:    []string{`C:\tools\codeql-lsp\codeql-lsp`},
			want:    true,
		},
		{
			name:    "non codeql command",
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
	got := describeCapabilityCommand("codeql.run", "CodeQL")
	if !strings.Contains(got, "CodeQL") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
