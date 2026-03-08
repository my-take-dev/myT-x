package thrift

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
			name:    "direct thrift ls command",
			command: "thrift-ls",
			want:    true,
		},
		{
			name:    "thriftls executable path",
			command: `C:\tools\thrift\thriftls.exe`,
			want:    true,
		},
		{
			name:    "npx launch with software mansion thrift ls package",
			command: "npx",
			args:    []string{"@software-mansion/thrift-ls"},
			want:    true,
		},
		{
			name:    "go launch with ocfbnj thrift ls module",
			command: "go",
			args:    []string{"run", "github.com/ocfbnj/thrift-ls"},
			want:    true,
		},
		{
			name:    "unrelated command",
			command: "thrift",
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
	got := describeCapabilityCommand("thrift.generateBindings", "Thrift")
	if !strings.Contains(got, "Thrift") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
