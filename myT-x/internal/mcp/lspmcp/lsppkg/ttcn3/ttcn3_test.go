package ttcn3

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
			name:    "direct ntt command",
			command: "ntt",
			want:    true,
		},
		{
			name:    "direct titan language server command",
			command: "titan-language-server",
			want:    true,
		},
		{
			name:    "java launch with ttcn3 language server jar",
			command: "java",
			args:    []string{"-jar", `/opt/ttcn3/ttcn3-language-server.jar`},
			want:    true,
		},
		{
			name:    "wrapper args contain ntt language server path",
			command: "wrapper",
			args:    []string{"--server", `C:\tools\ntt-language-server\ntt-language-server.exe`},
			want:    true,
		},
		{
			name:    "ntt docs command should not match",
			command: "nttdoc",
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
	got := describeCapabilityCommand("ttcn3.rebuildProject", "TTCN-3")
	if !strings.Contains(got, "TTCN-3") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
