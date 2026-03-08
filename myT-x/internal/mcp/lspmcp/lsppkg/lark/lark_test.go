package lark

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
			name:    "direct lark parser language server command from mcp list",
			command: "lark-parser-language-server",
			want:    true,
		},
		{
			name:    "lark parser language server executable path",
			command: `C:\tools\lark-parser-language-server\lark-parser-language-server.exe`,
			want:    true,
		},
		{
			name:    "python module launch with lark parser language server",
			command: "python",
			args:    []string{"-m", "lark_parser_language_server"},
			want:    true,
		},
		{
			name:    "python launch with lark parser language server script path",
			command: "python3",
			args:    []string{`/opt/lark-parser-language-server/server.py`},
			want:    true,
		},
		{
			name:    "wrapper args contain lark parser language server path",
			command: "wrapper",
			args:    []string{"--server", `/usr/local/bin/lark-parser-language-server`},
			want:    true,
		},
		{
			name:    "python launch with unrelated module should not match",
			command: "python",
			args:    []string{"-m", "pylsp"},
			want:    false,
		},
		{
			name:    "unrelated command",
			command: "texlab",
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
	got := describeCapabilityCommand("workspace/executeCommand", "Lark")
	if !strings.Contains(got, "Lark") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
