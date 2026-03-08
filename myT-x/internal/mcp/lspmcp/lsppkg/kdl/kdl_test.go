package kdl

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
			name:    "direct vscode kdl command from mcp list",
			command: "vscode-kdl",
			want:    true,
		},
		{
			name:    "vscode kdl executable path",
			command: `C:\tools\vscode-kdl\vscode-kdl.cmd`,
			want:    true,
		},
		{
			name:    "node launch with vscode kdl arg",
			command: "node",
			args:    []string{`/opt/extensions/vscode-kdl/out/server.js`},
			want:    true,
		},
		{
			name:    "arg contains kdl language server path",
			command: "wrapper",
			args:    []string{`C:\servers\kdl-language-server\bin\server.js`},
			want:    true,
		},
		{
			name:    "node launch with json language server should not match",
			command: "node",
			args:    []string{`/opt/vscode-json-languageserver/out/jsonServerMain.js`},
			want:    false,
		},
		{
			name:    "json language server command should not match",
			command: "vscode-json-languageserver",
			want:    false,
		},
		{
			name:    "non kdl command",
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
	got := describeCapabilityCommand("kdl.format", "KDL")
	if !strings.Contains(got, "KDL") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
