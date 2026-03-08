package openvalidation

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
			name:    "direct ov language server command",
			command: "ov-language-server",
			want:    true,
		},
		{
			name:    "ov language server executable path",
			command: `C:\tools\ov-language-server\ov-language-server.cmd`,
			want:    true,
		},
		{
			name:    "node launch with ov language server arg",
			command: "node",
			args:    []string{`C:\tools\ov-language-server\dist\server.js`},
			want:    true,
		},
		{
			name:    "arg contains ov language server path",
			command: "wrapper",
			args:    []string{`C:\tools\openvalidation\ov-language-server`},
			want:    true,
		},
		{
			name:    "node launch with unrelated script",
			command: "node",
			args:    []string{`C:\tools\scripts\start.js`},
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
	got := describeCapabilityCommand("ov.validate", "openVALIDATION")
	if !strings.Contains(got, "openVALIDATION") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
