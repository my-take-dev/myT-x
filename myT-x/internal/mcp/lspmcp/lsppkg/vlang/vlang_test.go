package vlang

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
			name:    "direct v analyzer command",
			command: "v-analyzer",
			want:    true,
		},
		{
			name:    "v analyzer executable path",
			command: `C:\tools\v-analyzer\v-analyzer.exe`,
			want:    true,
		},
		{
			name:    "v cli launches v analyzer",
			command: "v",
			args:    []string{"run", "/opt/v-analyzer/cmd/v-analyzer.v"},
			want:    true,
		},
		{
			name:    "wrapper args contain v analyzer path",
			command: "wrapper",
			args:    []string{"/usr/local/bin/v-analyzer"},
			want:    true,
		},
		{
			name:    "v cli without analyzer reference",
			command: "v",
			args:    []string{"fmt", "main.v"},
			want:    false,
		},
		{
			name:    "plain v command should not match",
			command: "v",
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
	got := describeCapabilityCommand("workspace/executeCommand", "V")
	if !strings.Contains(got, "V") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
