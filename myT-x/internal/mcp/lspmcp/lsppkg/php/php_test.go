package php

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
			name:    "direct crane command from mcp list",
			command: "crane",
			want:    true,
		},
		{
			name:    "direct intelephense command from mcp list",
			command: "intelephense",
			want:    true,
		},
		{
			name:    "direct php language server script from mcp list",
			command: "php-language-server.php",
			want:    true,
		},
		{
			name:    "php runtime with php language server script path",
			command: "php",
			args:    []string{"/opt/php-language-server/bin/php-language-server.php", "--stdio"},
			want:    true,
		},
		{
			name:    "direct serenata command from mcp list",
			command: "serenata",
			want:    true,
		},
		{
			name:    "node launch with intelephense script",
			command: "node",
			args:    []string{`C:\tools\vscode-intelephense\server\lib\intelephense.js`, "--stdio"},
			want:    true,
		},
		{
			name:    "npx launch with crane package",
			command: "npx",
			args:    []string{"@hvyindustries/crane", "--stdio"},
			want:    true,
		},
		{
			name:    "phan language server mode",
			command: "phan",
			args:    []string{"--language-server-on-stdin"},
			want:    true,
		},
		{
			name:    "phpactor language server subcommand",
			command: "phpactor",
			args:    []string{"language-server"},
			want:    true,
		},
		{
			name:    "wrapper command with phpactor binary and language server mode",
			command: "wrapper",
			args:    []string{"/usr/local/bin/phpactor", "language-server"},
			want:    true,
		},
		{
			name:    "php runtime with regular script does not match",
			command: "php",
			args:    []string{"artisan", "serve"},
			want:    false,
		},
		{
			name:    "phan static analysis invocation does not match",
			command: "phan",
			args:    []string{"--no-progress-bar", "--allow-polyfill-parser"},
			want:    false,
		},
		{
			name:    "phpactor non language server subcommand does not match",
			command: "phpactor",
			args:    []string{"status"},
			want:    false,
		},
		{
			name:    "phpunit language server should not match php package",
			command: "phpunit-language-server",
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
	got := describeCapabilityCommand("phpactor.index.references", "PHP")
	if !strings.Contains(got, "PHP") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
