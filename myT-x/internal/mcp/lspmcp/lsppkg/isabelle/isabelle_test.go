package isabelle

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
			name:    "direct isabelle vscode_server launch",
			command: "isabelle",
			args:    []string{"vscode_server"},
			want:    true,
		},
		{
			name:    "isabelle executable path with vscode_server",
			command: `C:\isabelle\bin\isabelle.exe`,
			args:    []string{"vscode_server", "-o", "vscode_pide_extensions=true"},
			want:    true,
		},
		{
			name:    "scala launch from isabelle sources tree",
			command: "scala",
			args:    []string{"-cp", "/opt/isabelle/src/Tools/VSCode/lib/*", "isabelle.vscode.Language_Server"},
			want:    true,
		},
		{
			name:    "java launch with isabelle vscode server class",
			command: "java",
			args:    []string{"-cp", "/opt/isabelle/lib/classes", "isabelle.vscode.server"},
			want:    true,
		},
		{
			name:    "arg contains isabelle vscode server binary path",
			command: "wrapper",
			args:    []string{`C:\isabelle\tools\isabelle-vscode-server.exe`},
			want:    true,
		},
		{
			name:    "isabelle build command should not match",
			command: "isabelle",
			args:    []string{"build", "-D", "."},
			want:    false,
		},
		{
			name:    "non isabelle command",
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
	got := describeCapabilityCommand("isabelle.loadTheory", "Isabelle")
	if !strings.Contains(got, "Isabelle") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
