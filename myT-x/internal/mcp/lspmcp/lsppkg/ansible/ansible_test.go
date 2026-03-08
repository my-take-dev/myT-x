package ansible

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
			name:    "direct ansible language server command",
			command: "ansible-language-server",
			want:    true,
		},
		{
			name:    "ansible language server executable path",
			command: `C:\tools\ansible-language-server\ansible-language-server.exe`,
			want:    true,
		},
		{
			name:    "node launch with ansible language server argument",
			command: "node",
			args:    []string{`C:\tools\ansible-language-server\out\server.js`},
			want:    true,
		},
		{
			name:    "arg contains ansible language server path",
			command: "wrapper",
			args:    []string{`C:\tools\ansible-language-server\ansible-language-server.cmd`},
			want:    true,
		},
		{
			name:    "non ansible command",
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
	got := describeCapabilityCommand("ansible.runPlaybook", "Ansible")
	if !strings.Contains(got, "Ansible") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
