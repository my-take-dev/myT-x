package delphi

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
			name:    "direct delphi lsp command",
			command: "delphi-lsp",
			want:    true,
		},
		{
			name:    "delphi lsp executable path",
			command: `C:\tools\DelphiLSP\DelphiLSP.exe`,
			want:    true,
		},
		{
			name:    "dotnet launch with delphi lsp dll",
			command: "dotnet",
			args:    []string{`C:\tools\DelphiLSP\DelphiLSP.dll`},
			want:    true,
		},
		{
			name:    "mono launch with delphi language server",
			command: "mono",
			args:    []string{`/opt/delphi/delphi-language-server.exe`},
			want:    true,
		},
		{
			name:    "arg contains delphi lsp path",
			command: "wrapper",
			args:    []string{`C:\servers\delphi-language-server.exe`},
			want:    true,
		},
		{
			name:    "non delphi command",
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
	got := describeCapabilityCommand("delphi.build", "Delphi")
	if !strings.Contains(got, "Delphi") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
