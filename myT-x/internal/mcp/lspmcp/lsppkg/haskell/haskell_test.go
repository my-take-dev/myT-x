package haskell

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
			name:    "direct haskell language server command",
			command: "haskell-language-server",
			want:    true,
		},
		{
			name:    "haskell language server wrapper executable path",
			command: `C:\tools\hls\haskell-language-server-wrapper.exe`,
			want:    true,
		},
		{
			name:    "arg contains haskell language server path",
			command: "wrapper",
			args:    []string{`C:\tools\hls\haskell-language-server-9.8.1.exe`},
			want:    true,
		},
		{
			name:    "non haskell command",
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
	got := describeCapabilityCommand("hls.eval", "Haskell")
	if !strings.Contains(got, "Haskell") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
