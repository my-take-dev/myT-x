package sysml2

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
			name:    "direct sysml2 language server command",
			command: "sysml2-language-server",
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
			args:    []string{`C:\repo\sysml2-tools\dist\sysml2-language-server.js`},
			want:    true,
		},
		{
			name:    "npx launch with sysml2 package",
			command: "npx",
			args:    []string{"@sensmetry/sysml2-tools", "lsp"},
			want:    true,
		},
		{
			name:    "arg contains sysml language server path",
			command: "wrapper",
			args:    []string{`/opt/sysml-language-server/bin/sysml-language-server`},
			want:    true,
		},
		{
			name:    "node launch with unrelated package",
			command: "node",
			args:    []string{`C:\repo\index.js`},
			want:    false,
		},
		{
			name:    "non sysml2 command",
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
	got := describeCapabilityCommand("sysml2.indexModel", "SysML v2")
	if !strings.Contains(got, "SysML v2") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
