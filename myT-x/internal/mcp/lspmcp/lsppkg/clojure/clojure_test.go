package clojure

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
			name:    "direct clojure lsp command",
			command: "clojure-lsp",
			want:    true,
		},
		{
			name:    "clojure executable path",
			command: `C:\tools\clojure-lsp\clojure-lsp.exe`,
			want:    true,
		},
		{
			name:    "arg contains clojure lsp path",
			command: "wrapper",
			args:    []string{`C:\tools\clojure-lsp\clojure-lsp`},
			want:    true,
		},
		{
			name:    "non clojure command",
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
	got := describeCapabilityCommand("clojure.analyze", "Clojure")
	if !strings.Contains(got, "Clojure") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
