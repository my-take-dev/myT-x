package powerpc

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
			name:    "direct powerpc support command",
			command: "powerpc-support",
			want:    true,
		},
		{
			name:    "powerpc language server executable path",
			command: `C:\tools\powerpc\powerpc-language-server.exe`,
			want:    true,
		},
		{
			name:    "node launch with powerpc support script path",
			command: "node",
			args:    []string{`/opt/noah-fields/powerpc-support/dist/server.js`},
			want:    true,
		},
		{
			name:    "npx launch with powerpc support package",
			command: "npx",
			args:    []string{"powerpc-support", "--stdio"},
			want:    true,
		},
		{
			name:    "wrapper args contain powerpc lsp path",
			command: "wrapper",
			args:    []string{`/usr/local/bin/powerpc-lsp`},
			want:    true,
		},
		{
			name:    "node launch with unrelated server",
			command: "node",
			args:    []string{`/opt/arm64-lsp/server.js`},
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
	got := describeCapabilityCommand("powerpc.assemble", "PowerPC Assembly")
	if !strings.Contains(got, "PowerPC Assembly") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
