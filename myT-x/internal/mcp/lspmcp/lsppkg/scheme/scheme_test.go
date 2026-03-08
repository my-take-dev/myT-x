package scheme

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
			name:    "direct scheme langserver command",
			command: "scheme-langserver",
			want:    true,
		},
		{
			name:    "scheme language server executable path",
			command: `C:\tools\scheme\scheme-language-server.exe`,
			want:    true,
		},
		{
			name:    "wrapper args include scheme lsp path",
			command: "wrapper",
			args:    []string{`C:\tools\scheme-lsp\scheme-lsp.exe`},
			want:    true,
		},
		{
			name:    "node launch scheme langserver script",
			command: "node",
			args:    []string{`C:\tools\scheme-langserver\dist\index.js`},
			want:    true,
		},
		{
			name:    "node launch unrelated script",
			command: "node",
			args:    []string{`C:\tools\typescript-language-server\cli.js`},
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
	got := describeCapabilityCommand("scheme.runCommand", "Scheme")
	if !strings.Contains(got, "Scheme") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
