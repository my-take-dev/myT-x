package kerml

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
			name:    "direct kerml language server command",
			command: "kerml-language-server",
			want:    true,
		},
		{
			name:    "sysml2 lsp executable path",
			command: `C:\tools\sysml2\sysml2-lsp.exe`,
			want:    true,
		},
		{
			name:    "node launch with sysml2 tools entrypoint",
			command: "node",
			args:    []string{`C:\repo\sysml2-tools\dist\kerml-language-server.js`},
			want:    true,
		},
		{
			name:    "npx launch with sysml2 package",
			command: "npx",
			args:    []string{"@sensmetry/sysml2-tools", "lsp"},
			want:    true,
		},
		{
			name:    "arg contains kerml lsp path",
			command: "wrapper",
			args:    []string{`/opt/kerml-lsp/bin/kerml-lsp`},
			want:    true,
		},
		{
			name:    "node launch with unrelated package",
			command: "node",
			args:    []string{`C:\repo\index.js`},
			want:    false,
		},
		{
			name:    "non kerml command",
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
	got := describeCapabilityCommand("kerml.indexModel", "KerML")
	if !strings.Contains(got, "KerML") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
