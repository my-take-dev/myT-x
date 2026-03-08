package kconfig

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
			name:    "direct kconfig language server command",
			command: "kconfig-language-server",
			want:    true,
		},
		{
			name:    "kconfig language server executable path",
			command: `C:\tools\kconfig-language-server\kconfig-language-server.cmd`,
			want:    true,
		},
		{
			name:    "bash launch with kconfig language server script",
			command: "bash",
			args:    []string{`/opt/kconfig-language-server/scripts/kconfig-language-server`},
			want:    true,
		},
		{
			name:    "arg contains kconfig lsp path",
			command: "wrapper",
			args:    []string{`C:\servers\kconfig-lsp\kconfig-lsp.exe`},
			want:    true,
		},
		{
			name:    "bash launch with unrelated script should not match",
			command: "bash",
			args:    []string{"/opt/scripts/build-kernel.sh"},
			want:    false,
		},
		{
			name:    "kconfig tooling command should not match",
			command: "kconfig-conf",
			want:    false,
		},
		{
			name:    "non kconfig command",
			command: "clangd",
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
	got := describeCapabilityCommand("kconfig.reload", "Kconfig")
	if !strings.Contains(got, "Kconfig") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
