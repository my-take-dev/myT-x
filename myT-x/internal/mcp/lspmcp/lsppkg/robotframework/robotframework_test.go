package robotframework

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
			name:    "direct robotframework lsp command",
			command: "robotframework-lsp",
			want:    true,
		},
		{
			name:    "direct robotframework ls command",
			command: "robotframework_ls",
			want:    true,
		},
		{
			name:    "python runtime with robotframework ls module",
			command: "python",
			args:    []string{"-m", "robotframework_ls"},
			want:    true,
		},
		{
			name:    "node launch with robocorp robotframework lsp script",
			command: "node",
			args:    []string{`C:\tools\robocorp\robotframework-lsp\out\server.js`},
			want:    true,
		},
		{
			name:    "robotcode language server subcommand",
			command: "robotcode",
			args:    []string{"language-server"},
			want:    true,
		},
		{
			name:    "arg contains robotframework lsp executable path",
			command: "wrapper",
			args:    []string{`/opt/robotframework-lsp/bin/robotframework-lsp`},
			want:    true,
		},
		{
			name:    "robot cli command does not match",
			command: "robot",
			args:    []string{"tests"},
			want:    false,
		},
		{
			name:    "robotcode non server command does not match",
			command: "robotcode",
			args:    []string{"format"},
			want:    false,
		},
		{
			name:    "python runtime with regular script",
			command: "python",
			args:    []string{"run_tests.py"},
			want:    false,
		},
		{
			name:    "node launch with unrelated script",
			command: "node",
			args:    []string{`C:\tools\scripts\main.js`},
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
	got := describeCapabilityCommand("robotframework.runTest", "Robot Framework")
	if !strings.Contains(got, "Robot Framework") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
