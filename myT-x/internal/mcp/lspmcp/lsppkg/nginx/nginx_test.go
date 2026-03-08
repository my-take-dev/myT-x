package nginx

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
			name:    "direct nginx language server command",
			command: "nginx-language-server",
			want:    true,
		},
		{
			name:    "nginx language server executable path",
			command: `C:\tools\nginx-language-server\nginx-language-server.exe`,
			want:    true,
		},
		{
			name:    "node launch with nginx language server script",
			command: "node",
			args:    []string{`C:\tools\nginx-language-server\dist\server.js`},
			want:    true,
		},
		{
			name:    "wrapper args contain nginxls path",
			command: "wrapper",
			args:    []string{`/usr/local/bin/nginxls`},
			want:    true,
		},
		{
			name:    "node launch without nginx reference",
			command: "node",
			args:    []string{`C:\tools\yaml-language-server\out\server.js`},
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
	got := describeCapabilityCommand("nginx.reloadConfig", "Nginx")
	if !strings.Contains(got, "Nginx") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
