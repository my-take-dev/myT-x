package bicep

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
			name:    "direct bicep language server command",
			command: "bicep-language-server",
			want:    true,
		},
		{
			name:    "bicep executable path",
			command: `C:\tools\bicep-language-server\bicep-language-server.exe`,
			want:    true,
		},
		{
			name:    "dotnet launch with bicep langserver arg",
			command: "dotnet",
			args:    []string{`C:\tools\bicep\Bicep.LangServer.dll`},
			want:    true,
		},
		{
			name:    "arg contains bicep langserver path",
			command: "wrapper",
			args:    []string{`C:\tools\bicep-langserver\bicep-langserver.exe`},
			want:    true,
		},
		{
			name:    "non bicep command",
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
	got := describeCapabilityCommand("bicep.deploy", "Bicep")
	if !strings.Contains(got, "Bicep") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
