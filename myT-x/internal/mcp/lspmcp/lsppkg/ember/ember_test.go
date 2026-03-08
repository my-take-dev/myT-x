package ember

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
			name:    "direct ember language server command",
			command: "ember-language-server",
			want:    true,
		},
		{
			name:    "ember language server executable path",
			command: `C:\tools\ember\ember-language-server.exe`,
			want:    true,
		},
		{
			name:    "node launch with ember language server entrypoint",
			command: "node",
			args:    []string{`C:\tools\ember-language-server\dist\index.js`},
			want:    true,
		},
		{
			name:    "npx launch with ember language server package",
			command: "npx",
			args:    []string{"ember-language-server", "--stdio"},
			want:    true,
		},
		{
			name:    "arg contains ember language server path",
			command: "wrapper",
			args:    []string{`C:\servers\ember-language-server.cmd`},
			want:    true,
		},
		{
			name:    "non ember command",
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
	got := describeCapabilityCommand("ember.findReferences", "Ember")
	if !strings.Contains(got, "Ember") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
