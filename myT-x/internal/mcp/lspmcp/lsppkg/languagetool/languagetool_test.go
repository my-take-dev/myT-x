package languagetool

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
			name:    "direct languagetool command from mcp list",
			command: "languagetool",
			want:    true,
		},
		{
			name:    "direct languagetool languageserver command",
			command: "languagetool-languageserver",
			want:    true,
		},
		{
			name:    "direct ltex ls command from mcp list",
			command: "ltex-ls",
			want:    true,
		},
		{
			name:    "java launch with languagetool jar",
			command: "java",
			args:    []string{"-jar", `C:\tools\languagetool-languageserver\languagetool-languageserver.jar`},
			want:    true,
		},
		{
			name:    "java launch with ltex ls jar",
			command: "java",
			args:    []string{"-jar", `/opt/ltex-ls/ltex-ls.jar`},
			want:    true,
		},
		{
			name:    "wrapper args contain ltex ls binary",
			command: "wrapper",
			args:    []string{"--server", `/usr/local/bin/ltex-ls`},
			want:    true,
		},
		{
			name:    "java launch with unrelated jar should not match",
			command: "java",
			args:    []string{"-jar", "myapp.jar"},
			want:    false,
		},
		{
			name:    "unrelated command",
			command: "texlab",
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
	got := describeCapabilityCommand("workspace/executeCommand", "LanguageTool")
	if !strings.Contains(got, "LanguageTool") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
