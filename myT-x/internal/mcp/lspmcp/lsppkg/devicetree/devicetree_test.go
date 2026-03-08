package devicetree

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
			name:    "direct dts-lsp command",
			command: "dts-lsp",
			want:    true,
		},
		{
			name:    "devicetree language server executable path",
			command: `C:\tools\dts\devicetree-language-server.exe`,
			want:    true,
		},
		{
			name:    "node launch with dts-lsp entrypoint",
			command: "node",
			args:    []string{`C:\tools\dts-lsp\dist\server.js`},
			want:    true,
		},
		{
			name:    "npx launch with dts-lsp package",
			command: "npx",
			args:    []string{"dts-lsp", "--stdio"},
			want:    true,
		},
		{
			name:    "arg contains devicetree-lsp path",
			command: "wrapper",
			args:    []string{`C:\servers\devicetree-lsp.exe`},
			want:    true,
		},
		{
			name:    "non devicetree command",
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
	got := describeCapabilityCommand("dts.indexWorkspace", "devicetree")
	if !strings.Contains(got, "devicetree") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
