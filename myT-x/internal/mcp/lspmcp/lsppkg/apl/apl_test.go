package apl

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
			name:    "direct apl language server command",
			command: "apl-language-server",
			want:    true,
		},
		{
			name:    "apl executable path",
			command: `C:\tools\apl-language-server\apl-language-server.exe`,
			want:    true,
		},
		{
			name:    "arg contains apl language server path",
			command: "wrapper",
			args:    []string{`C:\tools\apl-language-server\apl-language-server`},
			want:    true,
		},
		{
			name:    "non apl command",
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
	got := describeCapabilityCommand("apl.eval", "APL")
	if !strings.Contains(got, "APL") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
