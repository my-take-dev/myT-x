package abap

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
			name:    "direct abaplint command",
			command: "abaplint",
			want:    true,
		},
		{
			name:    "abaplint executable path",
			command: `C:\tools\abaplint\abaplint.exe`,
			want:    true,
		},
		{
			name:    "node launch with abaplint argument",
			command: "node",
			args:    []string{`C:\tools\abaplint\server.js`},
			want:    true,
		},
		{
			name:    "arg contains abaplint path",
			command: "wrapper",
			args:    []string{`C:\tools\abaplint\abaplint-ls.exe`},
			want:    true,
		},
		{
			name:    "non abap command",
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
	got := describeCapabilityCommand("abaplint.runFix", "ABAP")
	if !strings.Contains(got, "ABAP | abaplint") {
		t.Fatalf("expected ABAP | abaplint context in description, got %q", got)
	}
	if !strings.HasPrefix(got, "when: ") || !strings.Contains(got, " args: ") || !strings.Contains(got, " effect: exec.") {
		t.Fatalf("expected triad format in description, got %q", got)
	}
}
