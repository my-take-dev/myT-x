package miniyaml

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
			name:    "direct miniyaml language server command",
			command: "miniyaml-language-server",
			want:    true,
		},
		{
			name:    "oraide executable path",
			command: `C:\tools\oraide\oraide.exe`,
			want:    true,
		},
		{
			name:    "dotnet launch with oraide assembly",
			command: "dotnet",
			args:    []string{`C:\tools\oraide\Oraide.LanguageServer.dll`},
			want:    true,
		},
		{
			name:    "wrapper args contain mini yaml server",
			command: "wrapper",
			args:    []string{"--server", "mini-yaml-language-server"},
			want:    true,
		},
		{
			name:    "dotnet launch without miniyaml reference",
			command: "dotnet",
			args:    []string{`C:\tools\other\Some.Other.Server.dll`},
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
	got := describeCapabilityCommand("miniyaml.syncSchemas", "MiniYAML")
	if !strings.Contains(got, "MiniYAML") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
