package camel

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
			name:    "direct camel language server command",
			command: "camel-language-server",
			want:    true,
		},
		{
			name:    "camel language server executable path",
			command: `C:\tools\camel-language-server\camel-language-server.exe`,
			want:    true,
		},
		{
			name:    "java launch with camel language server jar",
			command: "java",
			args:    []string{"-jar", `C:\tools\camel-language-server\camel-language-server.jar`},
			want:    true,
		},
		{
			name:    "arg contains camel lsp server path",
			command: "wrapper",
			args:    []string{`C:\tools\camel-lsp-server\camel-lsp-server.exe`},
			want:    true,
		},
		{
			name:    "non camel command",
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
	got := describeCapabilityCommand("camel.routeAssist", "Apache Camel")
	if !strings.Contains(got, "Apache Camel") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
