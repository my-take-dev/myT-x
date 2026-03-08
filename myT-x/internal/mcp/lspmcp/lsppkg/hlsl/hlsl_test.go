package hlsl

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
			name:    "direct hlsl tools language server command",
			command: "HlslTools.LanguageServer",
			want:    true,
		},
		{
			name:    "hlsl tools executable path",
			command: `C:\tools\HlslTools\HlslTools.LanguageServer.exe`,
			want:    true,
		},
		{
			name:    "dotnet launch with hlsl tools dll",
			command: "dotnet",
			args:    []string{`C:\tools\HlslTools\HlslTools.LanguageServer.dll`},
			want:    true,
		},
		{
			name:    "arg contains hlsl lsp path",
			command: "wrapper",
			args:    []string{`C:\servers\hlsl-lsp\hlsl-lsp.exe`},
			want:    true,
		},
		{
			name:    "dotnet unrelated command should not match",
			command: "dotnet",
			args:    []string{`C:\tools\Roslyn\csc.dll`},
			want:    false,
		},
		{
			name:    "non hlsl command",
			command: "shader-language-server",
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
	got := describeCapabilityCommand("hlsl.compileShader", "HLSL")
	if !strings.Contains(got, "HLSL") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
