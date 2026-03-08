package cobol

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
			name:    "direct cobol language server command",
			command: "cobol-language-server",
			want:    true,
		},
		{
			name:    "rech editor cobol executable path",
			command: `C:\tools\rech-editor-cobol\rech-editor-cobol.exe`,
			want:    true,
		},
		{
			name:    "java launch with cobol language support arg",
			command: "java",
			args:    []string{"-jar", `C:\tools\cobol-language-support\cobol-language-server.jar`},
			want:    true,
		},
		{
			name:    "arg contains zopeneditor path",
			command: "wrapper",
			args:    []string{`C:\tools\zopeneditor\zopeneditor-language-server.exe`},
			want:    true,
		},
		{
			name:    "non cobol command",
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
	got := describeCapabilityCommand("cobol.compile", "COBOL")
	if !strings.Contains(got, "COBOL") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
