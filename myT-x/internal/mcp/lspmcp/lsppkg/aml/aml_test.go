package aml

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
			name:    "direct aml language server command",
			command: "aml-language-server",
			want:    true,
		},
		{
			name:    "aml language server executable path",
			command: `C:\tools\aml-language-server\aml-language-server.exe`,
			want:    true,
		},
		{
			name:    "java launch with aml language server jar path",
			command: "java",
			args:    []string{"-jar", `C:\tools\aml-language-server\aml-language-server.jar`},
			want:    true,
		},
		{
			name:    "arg contains amf language server path",
			command: "wrapper",
			args:    []string{`C:\tools\amf-language-server\amf-language-server.exe`},
			want:    true,
		},
		{
			name:    "non aml command",
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
	got := describeCapabilityCommand("aml.validate", "AML")
	if !strings.Contains(got, "AML") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
