package racket

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
			name:    "direct racket-langserver command",
			command: "racket-langserver",
			want:    true,
		},
		{
			name:    "racket-language-server executable path",
			command: `C:\tools\racket-langserver\racket-language-server.exe`,
			want:    true,
		},
		{
			name:    "racket launch with racket-langserver module",
			command: "racket",
			args:    []string{"-l", "racket-langserver"},
			want:    true,
		},
		{
			name:    "arg contains racket-langserver path",
			command: "wrapper",
			args:    []string{`/opt/racket-langserver/bin/racket-langserver`},
			want:    true,
		},
		{
			name:    "racket launch with unrelated module",
			command: "racket",
			args:    []string{"-l", "typed/racket"},
			want:    false,
		},
		{
			name:    "racket runtime with regular script",
			command: "racket",
			args:    []string{"script.rkt"},
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
	got := describeCapabilityCommand("racket.expand", "Racket")
	if !strings.Contains(got, "Racket") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
