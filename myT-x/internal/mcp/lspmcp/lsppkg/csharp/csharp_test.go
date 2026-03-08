package csharp

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
			name:    "direct omnisharp command",
			command: "omnisharp",
			want:    true,
		},
		{
			name:    "direct csharp-ls command",
			command: "csharp-ls",
			want:    true,
		},
		{
			name:    "omnisharp executable path",
			command: `C:\tools\OmniSharp\OmniSharp.exe`,
			want:    true,
		},
		{
			name:    "languageserver.net executable path",
			command: `C:\tools\LanguageServer.NET\LanguageServer.NET.exe`,
			want:    true,
		},
		{
			name:    "dotnet roslyn language server",
			command: "dotnet",
			args:    []string{"C:\\sdk\\Microsoft.CodeAnalysis.LanguageServer.dll"},
			want:    true,
		},
		{
			name:    "dotnet languageserver.net",
			command: "dotnet",
			args:    []string{"C:\\servers\\LanguageServer.NET\\LanguageServer.NET.dll"},
			want:    true,
		},
		{
			name:    "arg contains csharp-language-server",
			command: "wrapper",
			args:    []string{`C:\bin\csharp-language-server.exe`},
			want:    true,
		},
		{
			name:    "non csharp command",
			command: "clangd",
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
	got := describeCapabilityCommand("omnisharp.runCodeAction", "C#")
	if !strings.Contains(got, "C#") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
