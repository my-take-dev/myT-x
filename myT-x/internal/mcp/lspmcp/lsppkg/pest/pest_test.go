package pest

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
			name:    "direct pest ide tools command",
			command: "pest-ide-tools",
			want:    true,
		},
		{
			name:    "pest language server executable path",
			command: `C:\tools\pest\pest-language-server.exe`,
			want:    true,
		},
		{
			name:    "cargo launch with pest ide tools package",
			command: "cargo",
			args:    []string{"run", "--package", "pest-ide-tools", "--", "lsp"},
			want:    true,
		},
		{
			name:    "arg contains pest ide tools path",
			command: "wrapper",
			args:    []string{`C:\src\pest-ide-tools\target\release\pest-ide-tools.exe`},
			want:    true,
		},
		{
			name:    "cargo launch with unrelated package",
			command: "cargo",
			args:    []string{"run", "--package", "pestfmt"},
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
	got := describeCapabilityCommand("pest.generateAst", "Pest")
	if !strings.Contains(got, "Pest") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
