package chapel

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
			name:    "direct chapel language server command",
			command: "chapel-language-server",
			want:    true,
		},
		{
			name:    "chapel executable path",
			command: `C:\tools\chapel-language-server\chapel-language-server.exe`,
			want:    true,
		},
		{
			name:    "python launch with chapel arg",
			command: "python",
			args:    []string{"-m", "chapel-language-server"},
			want:    true,
		},
		{
			name:    "arg contains chapel language server path",
			command: "wrapper",
			args:    []string{`C:\tools\chapel-language-server\chapel-language-server`},
			want:    true,
		},
		{
			name:    "non chapel command",
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
	got := describeCapabilityCommand("chapel.build", "Chapel")
	if !strings.Contains(got, "Chapel") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
