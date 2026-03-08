package systemtap

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
			name:    "direct systemtap lsp command",
			command: "systemtap-lsp",
			want:    true,
		},
		{
			name:    "stap language server executable path",
			command: `C:\tools\systemtap\stap-language-server.exe`,
			want:    true,
		},
		{
			name:    "systemtap cli in lsp mode",
			command: "stap",
			args:    []string{"lsp"},
			want:    true,
		},
		{
			name:    "systemtap cli with explicit server switch",
			command: "systemtap",
			args:    []string{"--language-server"},
			want:    true,
		},
		{
			name:    "wrapper command with systemtap language server arg",
			command: "wrapper",
			args:    []string{"/opt/systemtap/bin/systemtap-language-server"},
			want:    true,
		},
		{
			name:    "stap without language server mode",
			command: "stap",
			args:    []string{"-v"},
			want:    false,
		},
		{
			name:    "systemtap command without lsp mode",
			command: "systemtap",
			args:    []string{"--version"},
			want:    false,
		},
		{
			name:    "non systemtap command",
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
	got := describeCapabilityCommand("systemtap.probe.resolve", "Systemtap")
	if !strings.Contains(got, "Systemtap") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
