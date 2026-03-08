package ceylon

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
			name:    "direct ceylon language server command",
			command: "ceylon-language-server",
			want:    true,
		},
		{
			name:    "ceylon executable path",
			command: `C:\tools\vscode-ceylon\vscode-ceylon.exe`,
			want:    true,
		},
		{
			name:    "arg contains ceylon language server path",
			command: "wrapper",
			args:    []string{`C:\tools\ceylon-language-server\ceylon-language-server`},
			want:    true,
		},
		{
			name:    "non ceylon command",
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
	got := describeCapabilityCommand("ceylon.compile", "Ceylon")
	if !strings.Contains(got, "Ceylon") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
