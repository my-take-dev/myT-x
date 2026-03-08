package cssls

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
			name:    "direct vscode css language server command",
			command: "vscode-css-language-server",
			want:    true,
		},
		{
			name:    "css executable path",
			command: `C:\tools\css-languageserver\css-languageserver.cmd`,
			want:    true,
		},
		{
			name:    "node launch with css language server arg",
			command: "node",
			args:    []string{`C:\tools\vscode-css-language-server\out\cssServerMain.js`},
			want:    true,
		},
		{
			name:    "arg contains css language server path",
			command: "wrapper",
			args:    []string{`C:\tools\vscode-css-language-server\vscode-css-language-server`},
			want:    true,
		},
		{
			name:    "non css command",
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
	got := describeCapabilityCommand("cssls.validate", "CSS/LESS/SASS")
	if !strings.Contains(got, "CSS/LESS/SASS") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
