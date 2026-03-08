package datapack

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
			name:    "direct datapack language server command",
			command: "datapack-language-server",
			want:    true,
		},
		{
			name:    "data pack executable path",
			command: `C:\tools\datapack-language-server\datapack-language-server.cmd`,
			want:    true,
		},
		{
			name:    "node launch with datapack arg",
			command: "node",
			args:    []string{`C:\tools\datapack-language-server\dist\index.js`},
			want:    true,
		},
		{
			name:    "arg contains data-pack language server path",
			command: "wrapper",
			args:    []string{`C:\tools\data-pack-language-server\data-pack-language-server`},
			want:    true,
		},
		{
			name:    "non datapack command",
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
	got := describeCapabilityCommand("datapack.run", "Data Pack")
	if !strings.Contains(got, "Data Pack") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
