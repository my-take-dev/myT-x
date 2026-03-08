package apachedispatcher

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
			name:    "direct apache dispatcher config language server command",
			command: "apache-dispatcher-config-language-server",
			want:    true,
		},
		{
			name:    "apache dispatcher executable path",
			command: `C:\tools\apache-dispatcher-config\apache-dispatcher-config-language-server.exe`,
			want:    true,
		},
		{
			name:    "node launch with apache dispatcher arg",
			command: "node",
			args:    []string{`C:\tools\vscode-apache-dispatcher-config-language-support\dist\server.js`},
			want:    true,
		},
		{
			name:    "arg contains dispatcher config language path",
			command: "wrapper",
			args:    []string{`C:\tools\dispatcher-config-language-server\apache-dispatcher-config-language-server.exe`},
			want:    true,
		},
		{
			name:    "non apache dispatcher command",
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
	got := describeCapabilityCommand("apachedispatcher.validate", "Apache Dispatcher Config")
	if !strings.Contains(got, "Apache Dispatcher Config") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
