package rexx

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
			name:    "direct rexx language server command",
			command: "rexx-language-server",
			want:    true,
		},
		{
			name:    "direct zopeneditor rexx language server command",
			command: "zopeneditor-rexx-language-server",
			want:    true,
		},
		{
			name:    "java launch with zopeneditor rexx jar",
			command: "java",
			args:    []string{"-jar", "/opt/IBM.zopeneditor/rexx-language-server.jar"},
			want:    true,
		},
		{
			name:    "arg contains rexx language server path",
			command: "wrapper",
			args:    []string{`C:\tools\zopeneditor-rexx-language-server\server\zopeneditor-rexx-language-server.exe`},
			want:    true,
		},
		{
			name:    "java launch with jcl language server should not match rexx",
			command: "java",
			args:    []string{"-jar", "/opt/IBM.zopeneditor/jcl-language-server.jar"},
			want:    false,
		},
		{
			name:    "generic zopeneditor language server without rexx marker",
			command: "zopeneditor-language-server",
			want:    false,
		},
		{
			name:    "rexx interpreter command does not match",
			command: "rexx",
			args:    []string{"script.rexx"},
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
	got := describeCapabilityCommand("rexx.organizeImports", "IBM TSO/E REXX")
	if !strings.Contains(got, "IBM TSO/E REXX") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
