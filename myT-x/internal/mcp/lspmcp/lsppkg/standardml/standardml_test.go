package standardml

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
			name:    "direct millet command",
			command: "millet",
			want:    true,
		},
		{
			name:    "millet executable path",
			command: `C:\tools\millet\millet.exe`,
			want:    true,
		},
		{
			name:    "cargo launch with millet binary argument",
			command: "cargo",
			args:    []string{"run", "--bin", "millet", "--", "lsp"},
			want:    true,
		},
		{
			name:    "arg contains millet binary path",
			command: "wrapper",
			args:    []string{`/usr/local/bin/millet`},
			want:    true,
		},
		{
			name:    "non standard ml command",
			command: "ocamllsp",
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
	got := describeCapabilityCommand("standardml.applyFix", "Standard ML")
	if !strings.Contains(got, "Standard ML") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
