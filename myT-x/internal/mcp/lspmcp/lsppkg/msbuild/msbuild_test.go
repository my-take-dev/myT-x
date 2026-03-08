package msbuild

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
			name:    "direct msbuild language server command",
			command: "msbuild-language-server",
			want:    true,
		},
		{
			name:    "msbuild project tools language server executable path",
			command: `C:\tools\msbuild-project-tools-language-server\msbuild-project-tools-language-server.exe`,
			want:    true,
		},
		{
			name:    "node launch with vscode msbuild project tools server script",
			command: "node",
			args:    []string{`/opt/vscode-msbuild-project-tools/out/server.js`},
			want:    true,
		},
		{
			name:    "npx launch with msbuild project tools language server",
			command: "npx",
			args:    []string{"msbuild-project-tools-language-server", "--stdio"},
			want:    true,
		},
		{
			name:    "wrapper args contain msbuild project tools server path",
			command: "wrapper",
			args:    []string{"--server", `/usr/local/bin/msbuild-project-tools-lsp`},
			want:    true,
		},
		{
			name:    "plain msbuild CLI should not match",
			command: "msbuild",
			want:    false,
		},
		{
			name:    "node launch with unrelated server should not match",
			command: "node",
			args:    []string{`/opt/vscode-markdown-languageserver/out/serverMain.js`},
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
	got := describeCapabilityCommand("workspace/executeCommand", "MSBuild")
	if !strings.Contains(got, "MSBuild") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
