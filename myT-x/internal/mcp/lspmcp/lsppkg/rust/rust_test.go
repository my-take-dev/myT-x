package rust

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
			name:    "direct rust analyzer command",
			command: "rust-analyzer",
			want:    true,
		},
		{
			name:    "rust analyzer executable path",
			command: `C:\tools\rust-analyzer\rust-analyzer.exe`,
			want:    true,
		},
		{
			name:    "wrapper args include rust analyzer path",
			command: "wrapper",
			args:    []string{`C:\tools\rust-analyzer\rust-analyzer.exe`},
			want:    true,
		},
		{
			name:    "cargo launch rust analyzer",
			command: "cargo",
			args:    []string{"run", "--bin", "rust-analyzer"},
			want:    true,
		},
		{
			name:    "cargo launch unrelated binary",
			command: "cargo",
			args:    []string{"run", "--bin", "cargo-watch"},
			want:    false,
		},
		{
			name:    "unrelated command",
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
	got := describeCapabilityCommand("rust-analyzer.applySourceChange", "Rust")
	if !strings.Contains(got, "Rust") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
