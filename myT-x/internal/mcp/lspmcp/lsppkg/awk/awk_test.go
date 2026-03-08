package awk

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
			name:    "direct awk language server command",
			command: "awk-language-server",
			want:    true,
		},
		{
			name:    "awk executable path",
			command: `C:\tools\awk-language-server\awk-language-server.exe`,
			want:    true,
		},
		{
			name:    "node launch with awk language server arg",
			command: "node",
			args:    []string{`C:\tools\awk-language-server\out\server.js`},
			want:    true,
		},
		{
			name:    "arg contains awk language server path",
			command: "wrapper",
			args:    []string{`C:\tools\awk-language-server\awk-language-server.cmd`},
			want:    true,
		},
		{
			name:    "non awk command",
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
	got := describeCapabilityCommand("awk.lint", "AWK")
	if !strings.Contains(got, "AWK") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
