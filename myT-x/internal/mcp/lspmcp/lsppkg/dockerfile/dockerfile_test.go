package dockerfile

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
			name:    "direct docker-langserver command",
			command: "docker-langserver",
			want:    true,
		},
		{
			name:    "dockerfile language server executable path",
			command: `C:\tools\docker\dockerfile-language-server.exe`,
			want:    true,
		},
		{
			name:    "node launch with dockerfile language server entrypoint",
			command: "node",
			args:    []string{`C:\tools\dockerfile-language-server-nodejs\lib\server.js`},
			want:    true,
		},
		{
			name:    "npx launch with docker-langserver package",
			command: "npx",
			args:    []string{"docker-langserver", "--stdio"},
			want:    true,
		},
		{
			name:    "docker language server should not match",
			command: "node",
			args:    []string{`C:\tools\docker-language-server\server.js`},
			want:    false,
		},
		{
			name:    "non dockerfile command",
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
	got := describeCapabilityCommand("dockerfile.resolveImage", "Dockerfiles")
	if !strings.Contains(got, "Dockerfiles") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
