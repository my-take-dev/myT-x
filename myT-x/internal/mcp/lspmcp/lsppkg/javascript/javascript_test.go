package javascript

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
			name:    "direct quick lint js command",
			command: "quick-lint-js",
			want:    true,
		},
		{
			name:    "quick lint js executable path",
			command: `C:\tools\quick-lint-js\quick-lint-js.exe`,
			want:    true,
		},
		{
			name:    "direct quick lint js lsp command",
			command: "quick-lint-js-lsp",
			want:    true,
		},
		{
			name:    "arg contains quick lint js path",
			command: "wrapper",
			args:    []string{`/opt/quick-lint-js/bin/quick-lint-js`},
			want:    true,
		},
		{
			name:    "similarly named but different command",
			command: "quick-lint",
			want:    false,
		},
		{
			name:    "unrelated command",
			command: "eslint-lsp",
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
	got := describeCapabilityCommand("quick-lint-js.fixAll", "JavaScript")
	if !strings.Contains(got, "JavaScript") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
