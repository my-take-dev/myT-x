package qsharp

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
			name:    "direct qsharp language server command",
			command: "qsharp-language-server",
			want:    true,
		},
		{
			name:    "qsharp language server executable path",
			command: `C:\tools\qsharp\qsharp-language-server.exe`,
			want:    true,
		},
		{
			name:    "dotnet launch with qsharp language server assembly",
			command: "dotnet",
			args:    []string{`C:\sdk\Microsoft.Quantum.QsCompiler.LanguageServer.dll`},
			want:    true,
		},
		{
			name:    "arg contains qsharp ls path",
			command: "wrapper",
			args:    []string{`/opt/qsharp/bin/qsharp-ls`},
			want:    true,
		},
		{
			name:    "dotnet launch with unrelated dll",
			command: "dotnet",
			args:    []string{`C:\tools\Some.Other.Tool.dll`},
			want:    false,
		},
		{
			name:    "qsharp compiler command",
			command: "qsc",
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
	got := describeCapabilityCommand("qsharp.compile", "Q#")
	if !strings.Contains(got, "Q#") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
