package yara

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
			name:    "direct yara language server command",
			command: "yara-language-server",
			want:    true,
		},
		{
			name:    "yarals executable path",
			command: `C:\tools\yara\yarals.exe`,
			want:    true,
		},
		{
			name:    "python launch with yara language server module",
			command: "python",
			args:    []string{"-m", "yara-language-server"},
			want:    true,
		},
		{
			name:    "wrapper args contain avast yara language server repo hint",
			command: "wrapper",
			args:    []string{"--source", "avast/yara-language-server"},
			want:    true,
		},
		{
			name:    "python launch without yara reference",
			command: "python",
			args:    []string{"-m", "pylsp"},
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
	got := describeCapabilityCommand("yara.refreshRules", "YARA")
	if !strings.Contains(got, "YARA") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
