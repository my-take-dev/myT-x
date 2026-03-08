package vhdl

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
			name:    "direct vhdl ls command",
			command: "vhdl_ls",
			want:    true,
		},
		{
			name:    "direct sigasi lsp command",
			command: "sigasi-lsp",
			want:    true,
		},
		{
			name:    "direct vhdl for professionals server command",
			command: "vhdl-for-professionals-language-server",
			want:    true,
		},
		{
			name:    "java launch with sigasi lsp jar",
			command: "java",
			args:    []string{"-jar", "/opt/sigasi/sigasi-lsp.jar"},
			want:    true,
		},
		{
			name:    "dotnet launch with vhdl professionals language server dll",
			command: "dotnet",
			args:    []string{`C:\tools\VHDL.For.Professionals.LanguageServer\VHDL.For.Professionals.LanguageServer.dll`},
			want:    true,
		},
		{
			name:    "java without vhdl server reference",
			command: "java",
			args:    []string{"-version"},
			want:    false,
		},
		{
			name:    "sigasi command without lsp should not match",
			command: "sigasi",
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
	got := describeCapabilityCommand("workspace/executeCommand", "VHDL")
	if !strings.Contains(got, "VHDL") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
