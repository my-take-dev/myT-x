package elm

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
			name:    "direct elm language server command",
			command: "elm-language-server",
			want:    true,
		},
		{
			name:    "elm language server executable path",
			command: `C:\tools\elm\elm-language-server.exe`,
			want:    true,
		},
		{
			name:    "node launch with elm language server entrypoint",
			command: "node",
			args:    []string{`C:\tools\@elm-tooling\elm-language-server\out\index.js`},
			want:    true,
		},
		{
			name:    "npx launch with elm language server package",
			command: "npx",
			args:    []string{"elm-language-server", "--stdio"},
			want:    true,
		},
		{
			name:    "arg contains elm lsp path",
			command: "wrapper",
			args:    []string{`C:\servers\elm-lsp.exe`},
			want:    true,
		},
		{
			name:    "non elm command",
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
	got := describeCapabilityCommand("elm.organizeImports", "Elm")
	if !strings.Contains(got, "Elm") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
