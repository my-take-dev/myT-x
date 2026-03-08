package kerboscript

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
			name:    "direct kos language server command",
			command: "kos-language-server",
			want:    true,
		},
		{
			name:    "kerboscript language server executable path",
			command: `C:\tools\kerboscript\kerboscript-language-server.exe`,
			want:    true,
		},
		{
			name:    "node launch with kos language server entrypoint",
			command: "node",
			args:    []string{`C:\tools\kos-language-server\dist\server.js`, "--stdio"},
			want:    true,
		},
		{
			name:    "npx launch with kos language server package",
			command: "npx",
			args:    []string{"kos-language-server", "--stdio"},
			want:    true,
		},
		{
			name:    "arg contains kos language server path",
			command: "wrapper",
			args:    []string{`/opt/kos-language-server/bin/kos-language-server`},
			want:    true,
		},
		{
			name:    "node launch with unrelated package",
			command: "node",
			args:    []string{`C:\repo\index.js`},
			want:    false,
		},
		{
			name:    "non kerboscript command",
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
	got := describeCapabilityCommand("kos.resolveImports", "KerboScript (kOS)")
	if !strings.Contains(got, "KerboScript (kOS)") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
