package efm

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
			name:    "direct efm command",
			command: "efm-langserver",
			want:    true,
		},
		{
			name:    "efm executable path",
			command: `C:\tools\efm\efm-langserver.exe`,
			want:    true,
		},
		{
			name:    "go run with efm module",
			command: "go",
			args:    []string{"run", "github.com/mattn/efm-langserver@latest"},
			want:    true,
		},
		{
			name:    "arg contains efm path",
			command: "wrapper",
			args:    []string{`/usr/local/bin/efm-langserver`},
			want:    true,
		},
		{
			name:    "go run unrelated module",
			command: "go",
			args:    []string{"run", "golang.org/x/tools/gopls@latest"},
			want:    false,
		},
		{
			name:    "non efm command",
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
	got := describeCapabilityCommand("efm.applyConfig", "EFM")
	if !strings.Contains(got, "EFM") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
