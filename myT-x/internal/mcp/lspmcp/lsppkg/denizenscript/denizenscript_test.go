package denizenscript

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
			name:    "direct denizen lsp command",
			command: "denizen-lsp",
			want:    true,
		},
		{
			name:    "denizenscript language server executable path",
			command: `C:\tools\denizen\denizenscript-language-server.exe`,
			want:    true,
		},
		{
			name:    "dotnet launch with denizen language server dll",
			command: "dotnet",
			args:    []string{`C:\tools\DenizenVSCode\DenizenLanguageServer.dll`},
			want:    true,
		},
		{
			name:    "mono launch with denizen language server",
			command: "mono",
			args:    []string{`/opt/denizen/DenizenLanguageServer.exe`},
			want:    true,
		},
		{
			name:    "arg contains denizenscript lsp path",
			command: "wrapper",
			args:    []string{`C:\servers\denizenscript-lsp.exe`},
			want:    true,
		},
		{
			name:    "non denizenscript command",
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
	got := describeCapabilityCommand("denizen.reloadScripts", "DenizenScript")
	if !strings.Contains(got, "DenizenScript") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
