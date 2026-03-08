package html

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
			name:    "direct vscode html language server command",
			command: "vscode-html-languageserver",
			want:    true,
		},
		{
			name:    "direct superhtml command",
			command: "superhtml",
			want:    true,
		},
		{
			name:    "node launch with vscode html language server arg",
			command: "node",
			args:    []string{`C:\tools\vscode-html-languageserver\bin\htmlServerMain.js`},
			want:    true,
		},
		{
			name:    "arg contains superhtml path",
			command: "wrapper",
			args:    []string{`C:\tools\superhtml\superhtml-lsp.exe`},
			want:    true,
		},
		{
			name:    "non html command",
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
	got := describeCapabilityCommand("html.doRename", "HTML")
	if !strings.Contains(got, "HTML") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
