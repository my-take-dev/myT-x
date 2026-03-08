package apielements

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
			name:    "direct api elements language server command",
			command: "apielements-language-server",
			want:    true,
		},
		{
			name:    "api elements executable path",
			command: `C:\tools\vscode-apielements\apielements-language-server.exe`,
			want:    true,
		},
		{
			name:    "node launch with vscode apielements arg",
			command: "node",
			args:    []string{`C:\tools\vscode-apielements\dist\server.js`},
			want:    true,
		},
		{
			name:    "arg contains api elements path",
			command: "wrapper",
			args:    []string{`C:\tools\apielements-language-server\apielements-language-server.exe`},
			want:    true,
		},
		{
			name:    "non api elements command",
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
	got := describeCapabilityCommand("apielements.preview", "API Elements")
	if !strings.Contains(got, "API Elements") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
