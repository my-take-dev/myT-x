package astro

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
			name:    "direct astro ls command",
			command: "astro-ls",
			want:    true,
		},
		{
			name:    "astro executable path",
			command: `C:\tools\astro-language-server\astro-language-server.exe`,
			want:    true,
		},
		{
			name:    "node launch with astro language server arg",
			command: "node",
			args:    []string{`C:\tools\@astrojs\language-server\dist\server.js`},
			want:    true,
		},
		{
			name:    "arg contains astro ls path",
			command: "wrapper",
			args:    []string{`C:\tools\astro-ls\bin\astro-ls`},
			want:    true,
		},
		{
			name:    "non astro command",
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
	got := describeCapabilityCommand("astro.organizeImports", "Astro")
	if !strings.Contains(got, "Astro") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
