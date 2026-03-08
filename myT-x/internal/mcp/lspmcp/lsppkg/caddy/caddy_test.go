package caddy

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
			name:    "direct caddyfile language server command",
			command: "caddyfile-language-server",
			want:    true,
		},
		{
			name:    "caddy executable path",
			command: `C:\tools\vscode-caddyfile\vscode-caddyfile.exe`,
			want:    true,
		},
		{
			name:    "node launch with caddy arg",
			command: "node",
			args:    []string{`C:\tools\vscode-caddyfile\dist\server.js`},
			want:    true,
		},
		{
			name:    "arg contains caddyfile language server path",
			command: "wrapper",
			args:    []string{`C:\tools\caddyfile-language-server\caddyfile-language-server`},
			want:    true,
		},
		{
			name:    "non caddy command",
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
	got := describeCapabilityCommand("caddy.validate", "caddy")
	if !strings.Contains(got, "caddy") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
