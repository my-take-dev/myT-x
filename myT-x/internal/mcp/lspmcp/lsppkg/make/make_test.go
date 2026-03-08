package makelsp

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
			name:    "direct make lsp command",
			command: "make-lsp",
			want:    true,
		},
		{
			name:    "direct make lsp vscode command",
			command: "make-lsp-vscode",
			want:    true,
		},
		{
			name:    "make language server executable path",
			command: `C:\tools\make-lsp\make-language-server.cmd`,
			want:    true,
		},
		{
			name:    "node launch with make lsp server path",
			command: "node",
			args:    []string{`/opt/make-lsp-vscode/out/server.js`, "--stdio"},
			want:    true,
		},
		{
			name:    "arg contains make lsp path",
			command: "wrapper",
			args:    []string{`C:\Users\dev\node_modules\make-lsp\dist\index.js`},
			want:    true,
		},
		{
			name:    "plain make command does not match",
			command: "make",
			want:    false,
		},
		{
			name:    "cmake language server does not match",
			command: "cmake-language-server",
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
	got := describeCapabilityCommand("make.resolveInclude", "Make")
	if !strings.Contains(got, "Make") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
