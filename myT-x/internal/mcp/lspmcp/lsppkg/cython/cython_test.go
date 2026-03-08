package cython

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
			name:    "direct cyright language server command",
			command: "cyright-langserver",
			want:    true,
		},
		{
			name:    "cyright executable path",
			command: `C:\tools\cyright\cyright.exe`,
			want:    true,
		},
		{
			name:    "node launch with cyright arg",
			command: "node",
			args:    []string{`C:\tools\cyright\dist\languageServer.js`},
			want:    true,
		},
		{
			name:    "arg contains cython language server path",
			command: "wrapper",
			args:    []string{`C:\tools\cython-language-server\cython-language-server`},
			want:    true,
		},
		{
			name:    "non cython command",
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
	got := describeCapabilityCommand("cython.run", "Cython")
	if !strings.Contains(got, "Cython") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
