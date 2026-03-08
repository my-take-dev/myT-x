package apex

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
			name:    "direct apex jorje lsp command",
			command: "apex-jorje-lsp",
			want:    true,
		},
		{
			name:    "apex executable path",
			command: `C:\tools\apex\apex-language-server.exe`,
			want:    true,
		},
		{
			name:    "java launch with apex jorje lsp jar",
			command: "java",
			args:    []string{"-jar", `C:\tools\apex-jorje-lsp\apex-jorje-lsp.jar`},
			want:    true,
		},
		{
			name:    "arg contains apex language server path",
			command: "wrapper",
			args:    []string{`C:\tools\apex-language-server\apex-language-server.exe`},
			want:    true,
		},
		{
			name:    "non apex command",
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
	got := describeCapabilityCommand("apex.runSymbol", "Apex")
	if !strings.Contains(got, "Apex") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
