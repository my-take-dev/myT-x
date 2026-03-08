package matlab

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
			name:    "direct matlab language server command from mcp list",
			command: "matlab-language-server",
			want:    true,
		},
		{
			name:    "matlab language server executable path",
			command: `C:\tools\matlab-language-server\matlab-language-server.exe`,
			want:    true,
		},
		{
			name:    "python module launch with matlab ls",
			command: "python",
			args:    []string{"-m", "matlab_ls"},
			want:    true,
		},
		{
			name:    "python launch with matlab language server script path",
			command: "py",
			args:    []string{`/opt/python-matlab-language-server/server.py`},
			want:    true,
		},
		{
			name:    "wrapper args contain matlab language server path",
			command: "wrapper",
			args:    []string{"--server", `/usr/local/bin/matlab-language-server`},
			want:    true,
		},
		{
			name:    "python launch with unrelated module should not match",
			command: "python",
			args:    []string{"-m", "pylsp"},
			want:    false,
		},
		{
			name:    "unrelated command",
			command: "octave-language-server",
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
	got := describeCapabilityCommand("workspace/executeCommand", "MATLAB")
	if !strings.Contains(got, "MATLAB") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
