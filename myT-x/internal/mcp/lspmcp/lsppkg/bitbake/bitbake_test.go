package bitbake

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
			name:    "direct bitbake language server command",
			command: "bitbake-language-server",
			want:    true,
		},
		{
			name:    "bitbake executable path",
			command: `C:\tools\bitbake-language-server\bitbake-language-server.exe`,
			want:    true,
		},
		{
			name:    "node launch with bitbake ls arg",
			command: "node",
			args:    []string{`C:\tools\bitbake-ls\dist\server.js`},
			want:    true,
		},
		{
			name:    "arg contains bitbake ls path",
			command: "wrapper",
			args:    []string{`C:\tools\bitbake-ls\bitbake-ls.exe`},
			want:    true,
		},
		{
			name:    "non bitbake command",
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
	got := describeCapabilityCommand("bitbake.task", "BitBake")
	if !strings.Contains(got, "BitBake") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
