package boriel

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
			name:    "direct boriel basic lsp command",
			command: "boriel-basic-lsp",
			want:    true,
		},
		{
			name:    "boriel executable path",
			command: `C:\tools\boriel-basic-lsp\boriel-basic-lsp.exe`,
			want:    true,
		},
		{
			name:    "arg contains boriel path",
			command: "wrapper",
			args:    []string{`C:\tools\boriel-basic-lsp\boriel-basic-lsp`},
			want:    true,
		},
		{
			name:    "non boriel command",
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
	got := describeCapabilityCommand("boriel.check", "Boriel Basic")
	if !strings.Contains(got, "Boriel Basic") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
