package brighterscript

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
			name:    "direct brighterscript language server command",
			command: "brighterscript-language-server",
			want:    true,
		},
		{
			name:    "brighterscript executable path",
			command: `C:\tools\brighterscript-ls\brighterscript-ls.exe`,
			want:    true,
		},
		{
			name:    "node launch with brighterscript arg",
			command: "node",
			args:    []string{`C:\tools\brighterscript\dist\server.js`},
			want:    true,
		},
		{
			name:    "arg contains brighterscript path",
			command: "wrapper",
			args:    []string{`C:\tools\brighterscript\brighterscript.cmd`},
			want:    true,
		},
		{
			name:    "non brighterscript command",
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
	got := describeCapabilityCommand("brighterscript.compile", "BrightScript/BrighterScript")
	if !strings.Contains(got, "BrightScript/BrighterScript") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
