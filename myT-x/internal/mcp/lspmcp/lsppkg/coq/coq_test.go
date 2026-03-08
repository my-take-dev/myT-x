package coq

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
			name:    "direct coq lsp command",
			command: "coq-lsp",
			want:    true,
		},
		{
			name:    "vscoq executable path",
			command: `C:\tools\vscoq\vscoq-language-server.exe`,
			want:    true,
		},
		{
			name:    "arg contains coq lsp path",
			command: "wrapper",
			args:    []string{`C:\tools\coq-lsp\coq-lsp`},
			want:    true,
		},
		{
			name:    "non coq command",
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
	got := describeCapabilityCommand("coq.proof", "Coq")
	if !strings.Contains(got, "Coq") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
