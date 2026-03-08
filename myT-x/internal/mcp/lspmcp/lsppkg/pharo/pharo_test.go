package pharo

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
			name:    "direct pharo language server command",
			command: "pharo-language-server",
			want:    true,
		},
		{
			name:    "pharo language server executable path",
			command: `C:\tools\pharo\PharoLanguageServer.exe`,
			want:    true,
		},
		{
			name:    "wrapper args include pharo lsp path",
			command: "wrapper",
			args:    []string{`C:\tools\pharo\pharo-lsp.exe`},
			want:    true,
		},
		{
			name:    "pharo runtime launch with pharo language server script",
			command: "pharo",
			args:    []string{"--headless", "Pharo.image", "PharoLanguageServer.st"},
			want:    true,
		},
		{
			name:    "pharo runtime launch unrelated script",
			command: "pharo",
			args:    []string{"--headless", "Pharo.image", "Build.st"},
			want:    false,
		},
		{
			name:    "unrelated command",
			command: "metals",
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
	got := describeCapabilityCommand("pharo.workspaceCommand", "Smalltalk/Pharo")
	if !strings.Contains(got, "Smalltalk/Pharo") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
