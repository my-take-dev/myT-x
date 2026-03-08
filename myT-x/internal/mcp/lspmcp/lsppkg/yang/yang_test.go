package yang

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
			name:    "direct yang lsp command",
			command: "yang-lsp",
			want:    true,
		},
		{
			name:    "yang language server executable path",
			command: `C:\tools\yang\yang-language-server.exe`,
			want:    true,
		},
		{
			name:    "java launch with yang lsp artifact",
			command: "java",
			args:    []string{"-jar", `C:\tools\yang-lsp\yang-lsp.jar`},
			want:    true,
		},
		{
			name:    "wrapper args contain typefox yang lsp repo hint",
			command: "wrapper",
			args:    []string{"--source", "typefox/yang-lsp"},
			want:    true,
		},
		{
			name:    "java launch without yang reference",
			command: "java",
			args:    []string{"-jar", `C:\tools\lemminx\org.eclipse.lemminx-uber.jar`},
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
	got := describeCapabilityCommand("yang.reloadModules", "YANG")
	if !strings.Contains(got, "YANG") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
