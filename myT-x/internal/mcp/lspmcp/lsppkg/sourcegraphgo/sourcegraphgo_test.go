package sourcegraphgo

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
			name:    "direct go langserver command",
			command: "go-langserver",
			want:    true,
		},
		{
			name:    "sourcegraph go langserver executable path",
			command: `C:\tools\sourcegraph-go-langserver\go-langserver.exe`,
			want:    true,
		},
		{
			name:    "go run with sourcegraph go langserver module",
			command: "go",
			args:    []string{"run", "github.com/sourcegraph/go-langserver", "-mode=stdio"},
			want:    true,
		},
		{
			name:    "arg contains go langserver path",
			command: "wrapper",
			args:    []string{`C:\tools\go-langserver\go-langserver.exe`},
			want:    true,
		},
		{
			name:    "non sourcegraph go command",
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
	got := describeCapabilityCommand("workspace/symbol", "sourcegraph go-langserver")
	if !strings.Contains(got, "sourcegraph go-langserver") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
