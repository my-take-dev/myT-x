package markdown

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
			name:    "direct marksman command",
			command: "marksman",
			want:    true,
		},
		{
			name:    "marksman executable path",
			command: `C:\tools\marksman\marksman.exe`,
			want:    true,
		},
		{
			name:    "direct markmark command",
			command: "markmark",
			want:    true,
		},
		{
			name:    "direct markmark language server command",
			command: "markmark-language-server",
			want:    true,
		},
		{
			name:    "direct vscode markdown language server command",
			command: "vscode-markdown-languageserver",
			want:    true,
		},
		{
			name:    "node launch with vscode markdown language server script",
			command: "node",
			args:    []string{`/opt/vscode-markdown-languageserver/out/serverMain.js`},
			want:    true,
		},
		{
			name:    "npx launch with markmark",
			command: "npx",
			args:    []string{"markmark", "--stdio"},
			want:    true,
		},
		{
			name:    "arg contains marksman path",
			command: "wrapper",
			args:    []string{`/usr/local/bin/marksman`},
			want:    true,
		},
		{
			name:    "markdownlint command does not match",
			command: "markdownlint",
			want:    false,
		},
		{
			name:    "vscode html server does not match",
			command: "vscode-html-languageserver",
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
	got := describeCapabilityCommand("markdown.applyLinkRefactor", "Markdown")
	if !strings.Contains(got, "Markdown") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
