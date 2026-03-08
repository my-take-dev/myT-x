package bsl

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
			name:    "direct bsl-language-server command",
			command: "bsl-language-server",
			want:    true,
		},
		{
			name:    "absolute bsl-language-server executable path",
			command: `C:\tools\bsl-language-server\bsl-language-server.exe`,
			want:    true,
		},
		{
			name:    "java jar launch with bsl-language-server in args",
			command: "java",
			args:    []string{"-jar", `C:\tools\bsl-language-server\bsl-language-server.jar`},
			want:    true,
		},
		{
			name:    "arg contains bsl-language-server path",
			command: "wrapper",
			args:    []string{`C:\tools\bsl-language-server\bsl-language-server.cmd`},
			want:    true,
		},
		{
			name:    "non bsl command",
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
	got := describeCapabilityCommand("bsl.runCommand")
	want := `when: Run the "bsl.runCommand" workspace command only when BSL Language Server (1c-syntax/bsl-language-server) advertises it for 1C Enterprise (BSL); semantics and arguments are command-specific. args: command-specific payload effect: exec.`
	if got != want {
		t.Fatalf("describeCapabilityCommand() = %q, want %q", got, want)
	}
}

func TestDescribeCapabilityWhenWithoutCommandName(t *testing.T) {
	got := describeCapabilityWhen("")
	if strings.Contains(strings.ToLower(got), "connected language server") {
		t.Fatalf("expected bsl language server specific wording, got %q", got)
	}
	if !strings.Contains(got, "BSL Language Server (1c-syntax/bsl-language-server)") {
		t.Fatalf("expected bsl language server context in wording, got %q", got)
	}
}
