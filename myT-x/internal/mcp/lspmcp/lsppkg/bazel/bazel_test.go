package bazel

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
			name:    "direct bazel lsp command",
			command: "bazel-lsp",
			want:    true,
		},
		{
			name:    "bazel lsp executable path",
			command: `C:\tools\bazel-lsp\bazel-lsp.exe`,
			want:    true,
		},
		{
			name:    "arg contains bazel lsp path",
			command: "wrapper",
			args:    []string{`C:\tools\bazel-lsp\bazel-lsp`},
			want:    true,
		},
		{
			name:    "non bazel command",
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
	got := describeCapabilityCommand("bazel.query", "Bazel")
	if !strings.Contains(got, "Bazel") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
