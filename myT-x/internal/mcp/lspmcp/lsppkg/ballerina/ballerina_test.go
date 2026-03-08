package ballerina

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
			name:    "direct ballerina language server command",
			command: "ballerina-language-server",
			want:    true,
		},
		{
			name:    "ballerina executable path",
			command: `C:\tools\ballerina-language-server\ballerina-language-server.exe`,
			want:    true,
		},
		{
			name:    "ballerina command with language server arg",
			command: "ballerina",
			args:    []string{"start-lang-server", `C:\tools\ballerina-ls\ballerina-ls.jar`},
			want:    true,
		},
		{
			name:    "arg contains ballerina language server path",
			command: "wrapper",
			args:    []string{`C:\tools\ballerina-ls\ballerina-ls.exe`},
			want:    true,
		},
		{
			name:    "non ballerina command",
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
	got := describeCapabilityCommand("ballerina.build", "Ballerina")
	if !strings.Contains(got, "Ballerina") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
