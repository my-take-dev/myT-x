package cmake

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
			name:    "direct cmake language server command",
			command: "cmake-language-server",
			want:    true,
		},
		{
			name:    "direct neocmakelsp command",
			command: "neocmakelsp",
			want:    true,
		},
		{
			name:    "python launch with cmake language server arg",
			command: "python",
			args:    []string{"-m", "cmake-language-server"},
			want:    true,
		},
		{
			name:    "arg contains neocmakelsp path",
			command: "wrapper",
			args:    []string{`C:\tools\neocmakelsp\neocmakelsp.exe`},
			want:    true,
		},
		{
			name:    "non cmake command",
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
	got := describeCapabilityCommand("cmake.configure", "CMake")
	if !strings.Contains(got, "CMake") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
