package lua

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
			name:    "direct lua-lsp command from mcp list",
			command: "lua-lsp",
			want:    true,
		},
		{
			name:    "direct lua-language-server command from mcp list",
			command: "lua-language-server",
			want:    true,
		},
		{
			name:    "direct LuaHelper command from mcp list",
			command: "LuaHelper",
			want:    true,
		},
		{
			name:    "luahelper executable path",
			command: `C:\tools\LuaHelper\LuaHelper.exe`,
			want:    true,
		},
		{
			name:    "arg contains lua-language-server path",
			command: "wrapper",
			args:    []string{`/opt/lua-language-server/bin/lua-language-server`},
			want:    true,
		},
		{
			name:    "lua runtime without lsp should not match",
			command: "lua",
			args:    []string{"-e", "print('hello')"},
			want:    false,
		},
		{
			name:    "non lua command",
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
	got := describeCapabilityCommand("lua.workspace.bytecode", "Lua")
	if !strings.Contains(got, "Lua") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
