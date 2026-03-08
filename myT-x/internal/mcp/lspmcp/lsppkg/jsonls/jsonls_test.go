package jsonls

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
			name:    "direct vscode json language server command",
			command: "vscode-json-languageserver",
			want:    true,
		},
		{
			name:    "json language server executable path",
			command: `C:\tools\vscode-json-languageserver\vscode-json-languageserver.cmd`,
			want:    true,
		},
		{
			name:    "node launch with vscode json language server arg",
			command: "node",
			args:    []string{`/opt/vscode-json-languageserver/bin/vscode-json-languageserver`},
			want:    true,
		},
		{
			name:    "arg contains vscode json language server path",
			command: "wrapper",
			args:    []string{`C:\extensions\vscode-json-language-server\out\jsonServerMain.js`},
			want:    true,
		},
		{
			name:    "node launch with unrelated language server should not match",
			command: "node",
			args:    []string{`/opt/vscode-yaml-language-server/out/yamlServerMain.js`},
			want:    false,
		},
		{
			name:    "jsonnet language server should not match",
			command: "jsonnet-language-server",
			want:    false,
		},
		{
			name:    "non json command",
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
	got := describeCapabilityCommand("json.format", "JSON")
	if !strings.Contains(got, "JSON") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
