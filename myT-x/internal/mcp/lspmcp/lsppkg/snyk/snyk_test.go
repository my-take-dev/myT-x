package snyk

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
			name:    "direct snyk ls command",
			command: "snyk-ls",
			want:    true,
		},
		{
			name:    "snyk language server executable path",
			command: `C:\tools\snyk-ls\snyk-language-server.exe`,
			want:    true,
		},
		{
			name:    "node launch with snyk ls script",
			command: "node",
			args:    []string{`C:\tools\@snyk\snyk-ls\dist\server.js`},
			want:    true,
		},
		{
			name:    "npx launch with snyk ls package",
			command: "npx",
			args:    []string{"snyk-ls", "--stdio"},
			want:    true,
		},
		{
			name:    "non snyk command",
			command: "sqls",
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
	got := describeCapabilityCommand("snyk.applyCodeAction", "Snyk")
	if !strings.Contains(got, "Snyk") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
