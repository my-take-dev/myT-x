package typecobol

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
			name:    "direct typecobol language server command",
			command: "typecobol-language-server",
			want:    true,
		},
		{
			name:    "dotnet launch with typecobol language server dll",
			command: "dotnet",
			args:    []string{`C:\tools\TypeCobol.Language.Server\TypeCobol.Language.Server.dll`},
			want:    true,
		},
		{
			name:    "mono launch with typecobol language server executable",
			command: "mono",
			args:    []string{`/opt/typecobol/TypeCobol.Language.Server.exe`},
			want:    true,
		},
		{
			name:    "wrapper args contain typecobol language server path",
			command: "wrapper",
			args:    []string{`--server`, `C:\servers\typecobol-language-server\start.cmd`},
			want:    true,
		},
		{
			name:    "dotnet launch without typecobol reference",
			command: "dotnet",
			args:    []string{"watch", "run"},
			want:    false,
		},
		{
			name:    "typecobol robot server should not match",
			command: "typecobol-language-server-robot",
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
	got := describeCapabilityCommand("workspace/executeCommand", "TypeCobol")
	if !strings.Contains(got, "TypeCobol") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
