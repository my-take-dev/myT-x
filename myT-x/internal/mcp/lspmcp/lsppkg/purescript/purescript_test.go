package purescript

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
			name:    "direct purescript language server command",
			command: "purescript-language-server",
			want:    true,
		},
		{
			name:    "purescript language server executable path",
			command: `C:\tools\purescript-language-server\purescript-language-server.exe`,
			want:    true,
		},
		{
			name:    "node launch with purescript language server script path",
			command: "node",
			args:    []string{`C:\tools\nwolverson\purescript-language-server\dist\index.js`},
			want:    true,
		},
		{
			name:    "arg contains purescript language server binary path",
			command: "wrapper",
			args:    []string{`/opt/purescript-language-server/bin/purescript-language-server`},
			want:    true,
		},
		{
			name:    "purescript compiler command",
			command: "purs",
			want:    false,
		},
		{
			name:    "node launch with unrelated script",
			command: "node",
			args:    []string{`C:\tools\scripts\main.js`},
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
	got := describeCapabilityCommand("purescript.organizeImports", "PureScript")
	if !strings.Contains(got, "PureScript") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
