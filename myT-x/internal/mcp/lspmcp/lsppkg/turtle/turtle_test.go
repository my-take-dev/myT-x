package turtle

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
			name:    "direct turtle language server command",
			command: "turtle-language-server",
			want:    true,
		},
		{
			name:    "java launch with turtle language server jar",
			command: "java",
			args:    []string{"-jar", `C:\tools\turtle-language-server\turtle-language-server.jar`},
			want:    true,
		},
		{
			name:    "wrapper args contain stardog turtle language server path",
			command: "wrapper",
			args:    []string{"--server", "/opt/stardog-union/turtle-language-server/bin/server"},
			want:    true,
		},
		{
			name:    "plain turtle command should not match",
			command: "turtle",
			want:    false,
		},
		{
			name:    "unrelated command",
			command: "sparql-language-server",
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
	got := describeCapabilityCommand("turtle.normalizePrefixes", "Turtle")
	if !strings.Contains(got, "Turtle") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
