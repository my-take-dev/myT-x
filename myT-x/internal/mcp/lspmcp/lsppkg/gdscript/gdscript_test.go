package gdscript

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
			name:    "direct godot command",
			command: "godot4",
			want:    true,
		},
		{
			name:    "godot language server executable path",
			command: `C:\tools\godot\godot-language-server.exe`,
			want:    true,
		},
		{
			name:    "godot launch with lsp port argument",
			command: "godot",
			args:    []string{"--headless", "--lsp-port", "6005"},
			want:    true,
		},
		{
			name:    "arg contains gdscript language server path",
			command: "wrapper",
			args:    []string{`C:\tools\gdscript-language-server\gdscript-language-server.exe`},
			want:    true,
		},
		{
			name:    "non gdscript command",
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
	got := describeCapabilityCommand("godot.reloadScripts", "GDScript")
	if !strings.Contains(got, "GDScript") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
