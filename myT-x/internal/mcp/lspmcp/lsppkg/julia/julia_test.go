package julia

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
			name:    "direct julia language server command from mcp list",
			command: "Julia language server",
			want:    true,
		},
		{
			name:    "julia language server executable path",
			command: `C:\tools\julia-language-server\julia-language-server.exe`,
			want:    true,
		},
		{
			name:    "julia runtime with LanguageServer.jl script",
			command: "julia",
			args:    []string{"--project=/opt/julia-ls", "/opt/julia-ls/src/LanguageServer.jl"},
			want:    true,
		},
		{
			name:    "julia runtime with inline using LanguageServer",
			command: "julia",
			args:    []string{"--startup-file=no", "-e", "using LanguageServer; using Sockets; runserver()"},
			want:    true,
		},
		{
			name:    "arg contains julia language server label",
			command: "wrapper",
			args:    []string{`C:\servers\Julia language server\bin\start.bat`},
			want:    true,
		},
		{
			name:    "julia runtime with unrelated script should not match",
			command: "julia",
			args:    []string{"-e", "using Pkg; Pkg.status()"},
			want:    false,
		},
		{
			name:    "non julia command",
			command: "python",
			args:    []string{"-m", "pylsp"},
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
	got := describeCapabilityCommand("julia.runCodeAction", "Julia")
	if !strings.Contains(got, "Julia") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
