package lpg

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
			name:    "direct lpg language server command",
			command: "LPG-language-server",
			want:    true,
		},
		{
			name:    "lpg language server executable path",
			command: `C:\tools\LPG-language-server\lpg-language-server.exe`,
			want:    true,
		},
		{
			name:    "arg contains lpg language server path",
			command: "wrapper",
			args:    []string{`/opt/lsp/LPG-language-server`},
			want:    true,
		},
		{
			name:    "arg contains lpg language server with underscore",
			command: "wrapper",
			args:    []string{`/opt/lsp/lpg_language_server`},
			want:    true,
		},
		{
			name:    "similarly named command does not match",
			command: "lspg-language-server",
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
	got := describeCapabilityCommand("lpg.generateParser", "LPG")
	if !strings.Contains(got, "LPG") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
