package angular

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
			name:    "direct ngserver command",
			command: "ngserver",
			want:    true,
		},
		{
			name:    "angular language server executable path",
			command: `C:\tools\angular-language-server\angular-language-server.exe`,
			want:    true,
		},
		{
			name:    "node launch with angular language server argument",
			command: "node",
			args:    []string{`C:\tools\@angular\language-server\bundles\index.js`},
			want:    true,
		},
		{
			name:    "arg contains ngserver path",
			command: "wrapper",
			args:    []string{`C:\tools\@angular\language-server\bin\ngserver`},
			want:    true,
		},
		{
			name:    "non angular command",
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
	got := describeCapabilityCommand("angular.findTemplate", "Angular")
	if !strings.Contains(got, "Angular") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
