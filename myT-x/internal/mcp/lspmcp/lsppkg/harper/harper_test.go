package harper

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
			name:    "direct harper ls command",
			command: "harper-ls",
			want:    true,
		},
		{
			name:    "direct harper command",
			command: "harper",
			want:    true,
		},
		{
			name:    "cargo run with harper package",
			command: "cargo",
			args:    []string{"run", "--package", "harper-ls", "--", "stdio"},
			want:    true,
		},
		{
			name:    "arg contains harper language server binary",
			command: "wrapper",
			args:    []string{`/usr/local/bin/harper-language-server`},
			want:    true,
		},
		{
			name:    "cargo run unrelated package",
			command: "cargo",
			args:    []string{"run", "--package", "ripgrep"},
			want:    false,
		},
		{
			name:    "non harper command",
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
	got := describeCapabilityCommand("harper.check", "Harper")
	if !strings.Contains(got, "Harper") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
