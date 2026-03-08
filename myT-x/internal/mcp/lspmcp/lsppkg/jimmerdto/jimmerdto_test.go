package jimmerdto

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
			name:    "direct jimmer dto lsp command",
			command: "jimmer-dto-lsp",
			want:    true,
		},
		{
			name:    "jimmer dto lsp executable path",
			command: `C:\tools\jimmer-dto-lsp\jimmer-dto-lsp.exe`,
			want:    true,
		},
		{
			name:    "java launch with jimmer dto lsp jar",
			command: "java",
			args:    []string{"-jar", `/opt/jimmer-dto-lsp/jimmer-dto-lsp.jar`},
			want:    true,
		},
		{
			name:    "arg contains jimmer dto lsp module path",
			command: "wrapper",
			args:    []string{"github.com/Enaium/jimmer-dto-lsp"},
			want:    true,
		},
		{
			name:    "java launch with unrelated jar",
			command: "java",
			args:    []string{"-jar", `/opt/tools/kotlin-language-server.jar`},
			want:    false,
		},
		{
			name:    "unrelated command",
			command: "kotlin-lsp",
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
	got := describeCapabilityCommand("jimmerdto.generate", "Jimmer DTO")
	if !strings.Contains(got, "Jimmer DTO") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
