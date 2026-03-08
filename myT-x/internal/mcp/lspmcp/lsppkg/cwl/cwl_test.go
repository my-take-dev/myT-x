package cwl

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
			name:    "direct cwl language server command",
			command: "cwl-language-server",
			want:    true,
		},
		{
			name:    "benten executable path",
			command: `C:\tools\benten\benten.exe`,
			want:    true,
		},
		{
			name:    "python launch with cwl language server arg",
			command: "python",
			args:    []string{"-m", "cwl-language-server"},
			want:    true,
		},
		{
			name:    "arg contains benten path",
			command: "wrapper",
			args:    []string{`C:\tools\benten\benten`},
			want:    true,
		},
		{
			name:    "non cwl command",
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
	got := describeCapabilityCommand("cwl.validate", "CWL")
	if !strings.Contains(got, "CWL") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
