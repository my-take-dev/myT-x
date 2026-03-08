package cucumber

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
			name:    "direct cucumber language server command",
			command: "cucumber-language-server",
			want:    true,
		},
		{
			name:    "gherkin language server executable path",
			command: `C:\tools\gherkin-language-server\gherkin-language-server.exe`,
			want:    true,
		},
		{
			name:    "node launch with cucumber language server arg",
			command: "node",
			args:    []string{`C:\tools\@cucumber\language-server\dist\server.js`},
			want:    true,
		},
		{
			name:    "arg contains cucumber language server path",
			command: "wrapper",
			args:    []string{`C:\tools\cucumber-language-server\cucumber-language-server`},
			want:    true,
		},
		{
			name:    "non cucumber command",
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
	got := describeCapabilityCommand("cucumber.run", "Cucumber/Gherkin")
	if !strings.Contains(got, "Cucumber/Gherkin") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
