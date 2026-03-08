package fsharp

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
			name:    "direct fsharp language server command",
			command: "fsharp-language-server",
			want:    true,
		},
		{
			name:    "direct fsautocomplete command",
			command: "fsautocomplete",
			want:    true,
		},
		{
			name:    "dotnet launch with fsautocomplete dll",
			command: "dotnet",
			args:    []string{`C:\tools\fsautocomplete\fsautocomplete.dll`},
			want:    true,
		},
		{
			name:    "dotnet launch with fsharp language server dll",
			command: "dotnet",
			args:    []string{`C:\tools\fsharp-language-server\fsharp-language-server.dll`},
			want:    true,
		},
		{
			name:    "arg contains fsautocomplete executable",
			command: "wrapper",
			args:    []string{`C:\servers\fsautocomplete.exe`},
			want:    true,
		},
		{
			name:    "dotnet with unrelated dll should not match",
			command: "dotnet",
			args:    []string{`C:\tools\other\server.dll`},
			want:    false,
		},
		{
			name:    "non fsharp command",
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
	got := describeCapabilityCommand("fsharp.organizeImports", "F#")
	if !strings.Contains(got, "F#") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
