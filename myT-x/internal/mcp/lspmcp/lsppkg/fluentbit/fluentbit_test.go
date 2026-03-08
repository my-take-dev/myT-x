package fluentbit

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
			name:    "direct fluent-bit-lsp command",
			command: "fluent-bit-lsp",
			want:    true,
		},
		{
			name:    "alias fluentbit-lsp executable path",
			command: `C:\tools\fluentbit-lsp\fluentbit-lsp.exe`,
			want:    true,
		},
		{
			name:    "cargo launch with fluent-bit-lsp manifest",
			command: "cargo",
			args:    []string{"run", "--manifest-path", `C:\src\fluent-bit-lsp\Cargo.toml`},
			want:    true,
		},
		{
			name:    "arg contains fluent-bit language server path",
			command: "wrapper",
			args:    []string{`C:\tools\fluent-bit-language-server\fluent-bit-language-server.exe`},
			want:    true,
		},
		{
			name:    "fluent-bit runtime should not match",
			command: "fluent-bit",
			want:    false,
		},
		{
			name:    "non fluent-bit command",
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
	got := describeCapabilityCommand("fluentbit.reloadConfig", "fluent-bit")
	if !strings.Contains(got, "fluent-bit") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
