package python

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
			name:    "direct ty command",
			command: "ty",
			want:    true,
		},
		{
			name:    "ruff ty subcommand",
			command: "ruff",
			args:    []string{"ty", "server"},
			want:    true,
		},
		{
			name:    "direct pydev language server command",
			command: "pydev-language-server",
			want:    true,
		},
		{
			name:    "arg contains pydevd python script",
			command: "wrapper",
			args:    []string{`C:\tools\PyDev.Debugger\pydevd.py`},
			want:    true,
		},
		{
			name:    "direct pyright language server command",
			command: "pyright-langserver",
			want:    true,
		},
		{
			name:    "node launch with pyright language server script",
			command: "node",
			args:    []string{`C:\tools\microsoft\pyright\dist\langserver.index.js`},
			want:    true,
		},
		{
			name:    "direct pyrefly command",
			command: "pyrefly",
			want:    true,
		},
		{
			name:    "direct basedpyright language server command",
			command: "basedpyright-langserver",
			want:    true,
		},
		{
			name:    "node launch with basedpyright script",
			command: "node",
			args:    []string{`C:\tools\basedpyright\dist\langserver.index.js`},
			want:    true,
		},
		{
			name:    "direct python lsp server command",
			command: "pylsp",
			want:    true,
		},
		{
			name:    "python runtime with jedi language server module",
			command: "python",
			args:    []string{"-m", "jedi_language_server"},
			want:    true,
		},
		{
			name:    "direct pylyzer command",
			command: "pylyzer",
			want:    true,
		},
		{
			name:    "direct zuban command",
			command: "zuban",
			want:    true,
		},
		{
			name:    "arg contains python lsp server path",
			command: "wrapper",
			args:    []string{`/opt/python-lsp-server/bin/pylsp`},
			want:    true,
		},
		{
			name:    "python runtime with regular script",
			command: "python",
			args:    []string{"script.py"},
			want:    false,
		},
		{
			name:    "ruff check command",
			command: "ruff",
			args:    []string{"check", "."},
			want:    false,
		},
		{
			name:    "node launch with unrelated script",
			command: "node",
			args:    []string{`C:\tools\scripts\main.js`},
			want:    false,
		},
		{
			name:    "windows py launcher with script",
			command: "py",
			args:    []string{"-3", "script.py"},
			want:    false,
		},
		{
			name:    "unrelated command with ty substring",
			command: "typst-lsp",
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
	got := describeCapabilityCommand("python.organizeImports", "Python")
	if !strings.Contains(got, "Python") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
