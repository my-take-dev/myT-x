package mcshader

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
			name:    "direct mcshader-lsp command",
			command: "mcshader-lsp",
			want:    true,
		},
		{
			name:    "mc-shader-lsp executable path",
			command: `C:\tools\mcshader\mc-shader-lsp.exe`,
			want:    true,
		},
		{
			name:    "cargo launch with mcshader manifest",
			command: "cargo",
			args:    []string{"run", "--manifest-path", `C:\src\mcshader-lsp\Cargo.toml`},
			want:    true,
		},
		{
			name:    "arg contains mcshader language server path",
			command: "wrapper",
			args:    []string{`C:\tools\mcshader-language-server\mcshader-language-server.exe`},
			want:    true,
		},
		{
			name:    "non mcshader command",
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
	got := describeCapabilityCommand("mcshader.rebuildIndex", "GLSL for Minecraft")
	if !strings.Contains(got, "GLSL for Minecraft") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
