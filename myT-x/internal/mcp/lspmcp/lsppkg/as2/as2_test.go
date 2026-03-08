package as2

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
			name:    "direct as2 language server command",
			command: "as2-language-server",
			want:    true,
		},
		{
			name:    "as2 language server executable path",
			command: `C:\tools\vscode-as2\as2-language-server.exe`,
			want:    true,
		},
		{
			name:    "node launch with vscode-as2 argument",
			command: "node",
			args:    []string{`C:\tools\vscode-as2\dist\server.js`},
			want:    true,
		},
		{
			name:    "arg contains as2 lsp path",
			command: "wrapper",
			args:    []string{`C:\tools\as2-lsp\as2-lsp.exe`},
			want:    true,
		},
		{
			name:    "non as2 command",
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
	got := describeCapabilityCommand("as2.applyFix", "ActionScript 2.0")
	if !strings.Contains(got, "ActionScript 2.0") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "AS2 Language Support") {
		t.Fatalf("expected AS2 Language Support context in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
	if !strings.HasPrefix(got, "when: ") || !strings.Contains(got, " args: ") || !strings.Contains(got, " effect: ") {
		t.Fatalf("expected triad format in description, got %q", got)
	}
}
