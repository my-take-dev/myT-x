package bprob

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
			name:    "direct b language server command",
			command: "b-language-server",
			want:    true,
		},
		{
			name:    "b language executable path",
			command: `C:\tools\b-language-server\b-language-server.exe`,
			want:    true,
		},
		{
			name:    "java launch with b language server arg",
			command: "java",
			args:    []string{"-jar", `C:\tools\b-language-server\b-language-server.jar`},
			want:    true,
		},
		{
			name:    "arg contains prob language server path",
			command: "wrapper",
			args:    []string{`C:\tools\prob-language-server\prob-language-server.exe`},
			want:    true,
		},
		{
			name:    "non bprob command",
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
	got := describeCapabilityCommand("bprob.prove", "B/ProB")
	if !strings.Contains(got, "B/ProB") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
