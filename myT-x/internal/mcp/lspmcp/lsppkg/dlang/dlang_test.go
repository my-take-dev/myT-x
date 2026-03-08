package dlang

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
			name:    "direct serve-d command",
			command: "serve-d",
			want:    true,
		},
		{
			name:    "direct dls executable path",
			command: `C:\tools\dls\dls.exe`,
			want:    true,
		},
		{
			name:    "dub launch with serve-d arg",
			command: "dub",
			args:    []string{"run", "serve-d", "--", "--stdio"},
			want:    true,
		},
		{
			name:    "arg contains dls path",
			command: "wrapper",
			args:    []string{`C:\tools\dls\dls`},
			want:    true,
		},
		{
			name:    "non d command",
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
	got := describeCapabilityCommand("dlang.run", "D")
	if !strings.Contains(got, "D") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
