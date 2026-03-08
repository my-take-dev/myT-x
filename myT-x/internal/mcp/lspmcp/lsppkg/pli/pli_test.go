package pli

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
			name:    "direct pli language server command",
			command: "pli-language-server",
			want:    true,
		},
		{
			name:    "zopeneditor pli language server executable path",
			command: `C:\tools\zopeneditor\pli\zopeneditor-pli-language-server.exe`,
			want:    true,
		},
		{
			name:    "java launch with pli language support jar",
			command: "java",
			args:    []string{"-jar", `C:\tools\zopeneditor\pli-language-support\pli-language-server.jar`},
			want:    true,
		},
		{
			name:    "wrapper arg contains zopeneditor language server in pli path",
			command: "wrapper",
			args:    []string{`/opt/zopeneditor/pli/zopeneditor-language-server`},
			want:    true,
		},
		{
			name:    "java launch with jcl language support jar",
			command: "java",
			args:    []string{"-jar", `C:\tools\zopeneditor\jcl-language-support\jcl-language-server.jar`},
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
	got := describeCapabilityCommand("pli.build", "IBM Enterprise PL/I for z/OS")
	if !strings.Contains(got, "IBM Enterprise PL/I for z/OS") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
