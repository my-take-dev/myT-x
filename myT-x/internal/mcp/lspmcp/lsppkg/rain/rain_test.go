package rain

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
			name:    "direct RainLanguageServer command",
			command: "RainLanguageServer",
			want:    true,
		},
		{
			name:    "rain-language-server executable path",
			command: `C:\tools\rain\bin\rain-language-server.exe`,
			want:    true,
		},
		{
			name:    "dotnet launch with RainLanguageServer assembly",
			command: "dotnet",
			args:    []string{`C:\tools\RainLanguageServer\RainLanguageServer.dll`},
			want:    true,
		},
		{
			name:    "arg contains rain language server path",
			command: "wrapper",
			args:    []string{`/opt/rainlanguageserver/bin/rainlanguageserver`},
			want:    true,
		},
		{
			name:    "dotnet launch with unrelated assembly",
			command: "dotnet",
			args:    []string{`C:\tools\OtherLanguageServer.dll`},
			want:    false,
		},
		{
			name:    "rain cli command should not match",
			command: "rain",
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
	got := describeCapabilityCommand("rain.indexWorkspace", "Rain")
	if !strings.Contains(got, "Rain") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
