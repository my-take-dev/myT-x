package gn

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
			name:    "direct gn language server command",
			command: "gn-language-server",
			want:    true,
		},
		{
			name:    "gnls executable path",
			command: `C:\tools\gn\gnls.exe`,
			want:    true,
		},
		{
			name:    "cargo launch with gn language server manifest",
			command: "cargo",
			args:    []string{"run", "--manifest-path", `C:\src\gn-language-server\Cargo.toml`},
			want:    true,
		},
		{
			name:    "arg contains gn language server path",
			command: "wrapper",
			args:    []string{`C:\tools\gn-language-server\gn-language-server.exe`},
			want:    true,
		},
		{
			name:    "non gn command",
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
	got := describeCapabilityCommand("gn.format", "GN")
	if !strings.Contains(got, "GN") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
