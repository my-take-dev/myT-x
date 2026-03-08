package commonlisp

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
			name:    "direct cl lsp command",
			command: "cl-lsp",
			want:    true,
		},
		{
			name:    "common lisp executable path",
			command: `C:\tools\cl-lsp\cl-lsp.cmd`,
			want:    true,
		},
		{
			name:    "node launch with cl lsp arg",
			command: "node",
			args:    []string{`C:\tools\cl-lsp\dist\server.js`},
			want:    true,
		},
		{
			name:    "arg contains cl lsp path",
			command: "wrapper",
			args:    []string{`C:\tools\cl-lsp\cl-lsp`},
			want:    true,
		},
		{
			name:    "non common lisp command",
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
	got := describeCapabilityCommand("commonlisp.eval", "Common Lisp")
	if !strings.Contains(got, "Common Lisp") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
