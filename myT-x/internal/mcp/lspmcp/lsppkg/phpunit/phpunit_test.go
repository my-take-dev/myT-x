package phpunit

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
			name:    "direct phpunit language server command from mcp list",
			command: "phpunit-language-server",
			want:    true,
		},
		{
			name:    "phpunit language server executable path",
			command: `C:\tools\phpunit-language-server\phpunit-language-server.cmd`,
			want:    true,
		},
		{
			name:    "node launch with phpunit language server script",
			command: "node",
			args:    []string{`/opt/phpunit-language-server/dist/index.js`, "--stdio"},
			want:    true,
		},
		{
			name:    "npx launch with phpunit language server package",
			command: "npx",
			args:    []string{"phpunit-language-server", "--stdio"},
			want:    true,
		},
		{
			name:    "arg contains recca phpunit language server path",
			command: "wrapper",
			args:    []string{`C:\tools\Recca0120\phpunit-language-server\bin\server.js`},
			want:    true,
		},
		{
			name:    "phpunit test runner command should not match",
			command: "phpunit",
			want:    false,
		},
		{
			name:    "php runtime with phpunit runner should not match",
			command: "php",
			args:    []string{"vendor/bin/phpunit"},
			want:    false,
		},
		{
			name:    "php language server should not match phpunit package",
			command: "php-language-server",
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
	got := describeCapabilityCommand("phpunit.test.peek", "PHPUnit")
	if !strings.Contains(got, "PHPUnit") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
