package emacslisp

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
			name:    "direct ellsp command",
			command: "ellsp",
			want:    true,
		},
		{
			name:    "ellsp executable path",
			command: `C:\tools\ellsp\ellsp.exe`,
			want:    true,
		},
		{
			name:    "eask launch with ellsp",
			command: "eask",
			args:    []string{"exec", "ellsp"},
			want:    true,
		},
		{
			name:    "emacs launch with ellsp script",
			command: "emacs",
			args:    []string{"--script", `C:\src\Ellsp\ellsp.el`},
			want:    true,
		},
		{
			name:    "arg contains ellsp script path",
			command: "wrapper",
			args:    []string{`C:\src\Ellsp\ellsp.el`},
			want:    true,
		},
		{
			name:    "non emacslisp command",
			command: "clojure-lsp",
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
	got := describeCapabilityCommand("ellsp.codeAction", "Emacs Lisp")
	if !strings.Contains(got, "Emacs Lisp") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
