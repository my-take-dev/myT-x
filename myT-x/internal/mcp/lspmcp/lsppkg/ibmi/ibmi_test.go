package ibmi

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
			name:    "direct ibmi languages command",
			command: "ibmi-languages",
			want:    true,
		},
		{
			name:    "ibmi executable path",
			command: `C:\tools\ibmi-languages\ibmi-language-server.exe`,
			want:    true,
		},
		{
			name:    "java launch with ibmi languages arg",
			command: "java",
			args:    []string{"-jar", `C:\tools\ibmi-languages\ibmi-languages.jar`},
			want:    true,
		},
		{
			name:    "arg contains ibmi language server path",
			command: "wrapper",
			args:    []string{`C:\tools\ibmi-languages\ibmi-language-server`},
			want:    true,
		},
		{
			name:    "non ibmi command",
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
	got := describeCapabilityCommand("ibmi.compile", "IBM i")
	if !strings.Contains(got, "IBM i") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
