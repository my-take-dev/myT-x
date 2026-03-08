package kedro

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
			name:    "direct kedro language server command",
			command: "kedro-language-server",
			want:    true,
		},
		{
			name:    "kedro language server executable path",
			command: `C:\tools\kedro\kedro-vscode-language-server.exe`,
			want:    true,
		},
		{
			name:    "python launch with kedro vscode module",
			command: "python3",
			args:    []string{"-m", "kedro_vscode.language_server"},
			want:    true,
		},
		{
			name:    "arg contains kedro vscode path",
			command: "wrapper",
			args:    []string{`C:\tools\kedro-vscode\bin\kedro-language-server.exe`},
			want:    true,
		},
		{
			name:    "python launch with non kedro module",
			command: "python",
			args:    []string{"-m", "http.server"},
			want:    false,
		},
		{
			name:    "non kedro command",
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
	got := describeCapabilityCommand("kedro.refreshPipelines", "Kedro")
	if !strings.Contains(got, "Kedro") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
