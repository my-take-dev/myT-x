package zig

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
			name:    "direct zls command",
			command: "zls",
			want:    true,
		},
		{
			name:    "zls executable path",
			command: `C:\tools\zig\zls.exe`,
			want:    true,
		},
		{
			name:    "zig launch with zls project reference",
			command: "zig",
			args:    []string{"build", "-Ddep=zigtools/zls"},
			want:    true,
		},
		{
			name:    "wrapper args contain zls path",
			command: "wrapper",
			args:    []string{"--server", `/usr/local/bin/zls`},
			want:    true,
		},
		{
			name:    "zig invocation without zls reference",
			command: "zig",
			args:    []string{"build", "project.zig"},
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
	got := describeCapabilityCommand("zig.rebuildIndex", "Zig")
	if !strings.Contains(got, "Zig") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
