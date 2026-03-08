package bake

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
			name:    "direct docker language server command",
			command: "docker-language-server",
			want:    true,
		},
		{
			name:    "docker language server executable path",
			command: `C:\tools\docker-language-server\docker-language-server.exe`,
			want:    true,
		},
		{
			name:    "arg contains docker language server path",
			command: "wrapper",
			args:    []string{`C:\tools\docker-language-server\docker-language-server`},
			want:    true,
		},
		{
			name:    "non bake command",
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
	got := describeCapabilityCommand("bake.buildx", "Bake")
	if !strings.Contains(got, "Bake") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
