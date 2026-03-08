package fortran

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
			name:    "direct fortran-language-server command",
			command: "fortran-language-server",
			want:    true,
		},
		{
			name:    "direct fortls executable path",
			command: `C:\tools\fortls\fortls.exe`,
			want:    true,
		},
		{
			name:    "python launch with fortran language server module",
			command: "python3",
			args:    []string{"-m", "fortran_language_server"},
			want:    true,
		},
		{
			name:    "py launch with fortls module",
			command: "py",
			args:    []string{"-m", "fortls"},
			want:    true,
		},
		{
			name:    "uvx launch with fortls",
			command: "uvx",
			args:    []string{"fortls", "--stdio"},
			want:    true,
		},
		{
			name:    "python non fortran module",
			command: "python",
			args:    []string{"main.py"},
			want:    false,
		},
		{
			name:    "non fortran command",
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
	got := describeCapabilityCommand("fortran.buildModuleGraph", "Fortran")
	if !strings.Contains(got, "Fortran") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
