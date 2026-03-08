package wxml

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
			name:    "direct wxml language server command",
			command: "wxml-languageserver",
			want:    true,
		},
		{
			name:    "wxml language server executable path",
			command: `C:\tools\wxml-languageserver\wxml-languageserver.cmd`,
			want:    true,
		},
		{
			name:    "node launch with wxml language server script",
			command: "node",
			args:    []string{`C:\tools\wxml-languageserver\lib\server.js`},
			want:    true,
		},
		{
			name:    "wrapper args contain wxml language server repo hint",
			command: "wrapper",
			args:    []string{"--source", "chemzqm/wxml-languageserver"},
			want:    true,
		},
		{
			name:    "node launch without wxml reference",
			command: "node",
			args:    []string{`C:\tools\yaml-language-server\out\server.js`},
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
	got := describeCapabilityCommand("wxml.reloadProject", "WXML")
	if !strings.Contains(got, "WXML") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
