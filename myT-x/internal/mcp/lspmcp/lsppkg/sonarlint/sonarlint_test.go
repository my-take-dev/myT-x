package sonarlint

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
			name:    "direct sonarlint language server command",
			command: "sonarlint-language-server",
			want:    true,
		},
		{
			name:    "java launch with sonarlint jar",
			command: "java",
			args:    []string{"-jar", `C:\tools\sonarlint-language-server.jar`},
			want:    true,
		},
		{
			name:    "arg contains sonarlint ls binary",
			command: "wrapper",
			args:    []string{`/opt/sonarlint-ls/bin/sonarlint-ls`},
			want:    true,
		},
		{
			name:    "java launch with unrelated jdtls jar",
			command: "java",
			args:    []string{"-jar", `C:\tools\jdt-language-server.jar`},
			want:    false,
		},
		{
			name:    "non sonarlint command",
			command: "sonar-scanner",
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
	got := describeCapabilityCommand("sonarlint.scan", "SonarLint")
	if !strings.Contains(got, "SonarLint") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
