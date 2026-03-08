package jcl

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
			name:    "direct jcl language server command",
			command: "jcl-language-server",
			want:    true,
		},
		{
			name:    "zopeneditor language server executable path",
			command: `C:\tools\zopeneditor\zopeneditor-language-server.exe`,
			want:    true,
		},
		{
			name:    "java launch with jcl language support jar",
			command: "java",
			args:    []string{"-jar", `C:\tools\zopeneditor\jcl-language-support\jcl-language-server.jar`},
			want:    true,
		},
		{
			name:    "wrapper arg contains zopeneditor jcl language server path",
			command: "wrapper",
			args:    []string{`/opt/zopeneditor/jcl/zopeneditor-jcl-language-server`},
			want:    true,
		},
		{
			name:    "java launch with cobol language server jar",
			command: "java",
			args:    []string{"-jar", `C:\tools\cobol-language-support\cobol-language-server.jar`},
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
	got := describeCapabilityCommand("jcl.build", "JCL")
	if !strings.Contains(got, "JCL") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
