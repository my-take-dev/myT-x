package haxe

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
			name:    "direct haxe language server command",
			command: "haxe-language-server",
			want:    true,
		},
		{
			name:    "haxe executable path",
			command: `C:\tools\haxe-language-server\haxe-language-server.cmd`,
			want:    true,
		},
		{
			name:    "node launch with haxe language server arg",
			command: "node",
			args:    []string{`C:\tools\haxe-language-server\bin\server.js`},
			want:    true,
		},
		{
			name:    "arg contains haxe language server path",
			command: "wrapper",
			args:    []string{`C:\tools\haxe-language-server\haxe-language-server.exe`},
			want:    true,
		},
		{
			name:    "non haxe command",
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
	got := describeCapabilityCommand("haxe.build", "Haxe")
	if !strings.Contains(got, "Haxe") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
