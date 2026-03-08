package openedgeabl

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
			name:    "direct abl language server command",
			command: "abl-language-server",
			want:    true,
		},
		{
			name:    "openedge abl language server executable path",
			command: `C:\tools\abl-lsp\openedge-abl-language-server.exe`,
			want:    true,
		},
		{
			name:    "java launch with abl lsp jar",
			command: "java",
			args:    []string{"-jar", `C:\tools\abl-lsp\abl-lsp.jar`},
			want:    true,
		},
		{
			name:    "wrapper args contain riverside abl lsp reference",
			command: "wrapper",
			args:    []string{`--server`, `/src/github.com/Riverside-Software/abl-lsp/server/abl-language-server`},
			want:    true,
		},
		{
			name:    "java launch without abl lsp reference",
			command: "java",
			args:    []string{"-jar", `/opt/tools/checkstyle.jar`},
			want:    false,
		},
		{
			name:    "unrelated command",
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
	got := describeCapabilityCommand("openedgeabl.compile", "OpenEdge ABL")
	if !strings.Contains(got, "OpenEdge ABL") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
