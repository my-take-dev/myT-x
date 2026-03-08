package glsl

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
			name:    "direct glsl-language-server command",
			command: "glsl-language-server",
			want:    true,
		},
		{
			name:    "glsl-lsp executable path",
			command: `C:\tools\glsl\glsl-lsp.exe`,
			want:    true,
		},
		{
			name:    "arg contains glslls path",
			command: "wrapper",
			args:    []string{`C:\tools\glslls\glslls.exe`},
			want:    true,
		},
		{
			name:    "glslang validator should not match",
			command: "glslangValidator",
			want:    false,
		},
		{
			name:    "non glsl command",
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
	got := describeCapabilityCommand("glsl.compileShader", "GLSL")
	if !strings.Contains(got, "GLSL") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
