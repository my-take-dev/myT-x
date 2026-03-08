package puppet

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
			name:    "direct puppet language server command",
			command: "puppet-languageserver",
			want:    true,
		},
		{
			name:    "puppet language server executable path",
			command: `C:\tools\puppet\puppet-language-server.exe`,
			want:    true,
		},
		{
			name:    "ruby launch with puppet language server script path",
			command: "ruby",
			args:    []string{`C:\tools\puppetlabs\puppet-languageserver\bin\puppet-languageserver`},
			want:    true,
		},
		{
			name:    "arg contains puppet language server command",
			command: "wrapper",
			args:    []string{"puppet-language-server", "--stdio"},
			want:    true,
		},
		{
			name:    "ruby with unrelated script",
			command: "ruby",
			args:    []string{"script.rb"},
			want:    false,
		},
		{
			name:    "puppet cli command",
			command: "puppet",
			args:    []string{"apply", "manifest.pp"},
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
	got := describeCapabilityCommand("puppet.compile", "Puppet")
	if !strings.Contains(got, "Puppet") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
