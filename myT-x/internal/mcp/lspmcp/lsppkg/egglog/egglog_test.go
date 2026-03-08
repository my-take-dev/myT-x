package egglog

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
			name:    "direct egglog language server command",
			command: "egglog-language-server",
			want:    true,
		},
		{
			name:    "egglog executable path",
			command: `C:\tools\egglog-language-server\egglog-language-server.exe`,
			want:    true,
		},
		{
			name:    "cargo launch with manifest path",
			command: "cargo",
			args:    []string{"run", "--manifest-path", `C:\src\egglog-language-server\Cargo.toml`},
			want:    true,
		},
		{
			name:    "arg contains egglog language server path",
			command: "wrapper",
			args:    []string{`C:\src\egglog-language-server\target\release\egglog-language-server.exe`},
			want:    true,
		},
		{
			name:    "non egglog command",
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
	got := describeCapabilityCommand("egglog.run", "Egglog")
	if !strings.Contains(got, "Egglog") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
