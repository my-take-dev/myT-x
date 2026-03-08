package antlr

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
			name:    "direct antlr language server command",
			command: "antlr-language-server",
			want:    true,
		},
		{
			name:    "antlr executable path",
			command: `C:\tools\antlr\antlrvsix.exe`,
			want:    true,
		},
		{
			name:    "dotnet launch with antlr arg",
			command: "dotnet",
			args:    []string{`C:\tools\antlr\AntlrVSIX.dll`},
			want:    true,
		},
		{
			name:    "arg contains antlr lsp path",
			command: "wrapper",
			args:    []string{`C:\tools\antlr-lsp\antlr-lsp.exe`},
			want:    true,
		},
		{
			name:    "non antlr command",
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
	got := describeCapabilityCommand("antlr.generateParser", "Antlr")
	if !strings.Contains(got, "Antlr") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
