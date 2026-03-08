package liquid

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
			name:    "direct theme check language server command",
			command: "theme-check-language-server",
			want:    true,
		},
		{
			name:    "theme check language server executable path",
			command: `C:\tools\theme-check\theme-check-language-server.exe`,
			want:    true,
		},
		{
			name:    "theme check cli with language server subcommand",
			command: "theme-check",
			args:    []string{"language-server"},
			want:    true,
		},
		{
			name:    "ruby launch with theme check language server script",
			command: "ruby",
			args:    []string{`C:\Ruby\gems\theme-check\exe\theme-check-language-server`},
			want:    true,
		},
		{
			name:    "bundle exec theme check lsp",
			command: "bundle",
			args:    []string{"exec", "theme-check", "lsp"},
			want:    true,
		},
		{
			name:    "arg contains theme check server path",
			command: "wrapper",
			args:    []string{`/opt/theme-check/bin/theme_check_language_server`},
			want:    true,
		},
		{
			name:    "theme check cli without lsp subcommand",
			command: "theme-check",
			args:    []string{"check", "templates/index.liquid"},
			want:    false,
		},
		{
			name:    "unrelated liquid server command",
			command: "liquid-language-server",
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
	got := describeCapabilityCommand("theme-check.applyFix", "Liquid")
	if !strings.Contains(got, "Liquid") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
