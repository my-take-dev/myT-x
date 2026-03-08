package nim

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
			name:    "direct nimlsp command",
			command: "nimlsp",
			want:    true,
		},
		{
			name:    "nimlsp executable path",
			command: `C:\tools\nimlsp\nimlsp.exe`,
			want:    true,
		},
		{
			name:    "nimble launch with nimlsp package",
			command: "nimble",
			args:    []string{"run", "nimlsp"},
			want:    true,
		},
		{
			name:    "nim launch with nimlsp source",
			command: "nim",
			args:    []string{"r", `/opt/nimlsp/src/nimlsp.nim`},
			want:    true,
		},
		{
			name:    "wrapper args contain nimlsp path",
			command: "wrapper",
			args:    []string{`--server`, `C:\servers\nimlsp\nimlsp.exe`},
			want:    true,
		},
		{
			name:    "nimble invocation without nimlsp reference",
			command: "nimble",
			args:    []string{"run", "nimsuggest"},
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
	got := describeCapabilityCommand("nim.projectCheck", "Nim")
	if !strings.Contains(got, "Nim") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
