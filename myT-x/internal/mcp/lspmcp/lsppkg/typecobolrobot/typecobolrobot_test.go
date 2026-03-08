package typecobolrobot

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
			name:    "direct language server robot command",
			command: "language-server-robot",
			want:    true,
		},
		{
			name:    "direct typecobol language server robot executable",
			command: `C:\tools\typecobol\typecobol-language-server-robot.exe`,
			want:    true,
		},
		{
			name:    "dotnet launch with typecobol robot dll",
			command: "dotnet",
			args:    []string{`C:\tools\TypeCobol.Language.Server.Robot\TypeCobol.Language.Server.Robot.dll`},
			want:    true,
		},
		{
			name:    "mono launch with typecobol robot executable",
			command: "mono",
			args:    []string{`/opt/typecobol/TypeCobol.Language.Server.Robot.exe`},
			want:    true,
		},
		{
			name:    "wrapper args contain typecobol robot path",
			command: "wrapper",
			args:    []string{`--server`, `C:\servers\TypeCobol Language Server Robot\start.cmd`},
			want:    true,
		},
		{
			name:    "dotnet launch without typecobol robot reference",
			command: "dotnet",
			args:    []string{"watch", "run"},
			want:    false,
		},
		{
			name:    "language server robot without typecobol should not match",
			command: "robot-language-server",
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
	got := describeCapabilityCommand("workspace/executeCommand", "TypeCobol Language Server Robot")
	if !strings.Contains(got, "TypeCobol Language Server Robot") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
