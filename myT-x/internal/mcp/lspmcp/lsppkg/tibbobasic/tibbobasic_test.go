package tibbobasic

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
			name:    "direct tibbo basic ls command",
			command: "tibbo-basic-ls",
			want:    true,
		},
		{
			name:    "node launch with tibbo basic language server script",
			command: "node",
			args:    []string{`C:\tools\tibbo-basic-language-server\server.js`},
			want:    true,
		},
		{
			name:    "wrapper args contain tibbo basic repository path",
			command: "wrapper",
			args:    []string{"--server", "github.com/tibbo/tibbo-basic/lsp"},
			want:    true,
		},
		{
			name:    "tibbo basic compiler should not match",
			command: "tibbo-basic-compiler",
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
	got := describeCapabilityCommand("tibbobasic.compile", "Tibbo Basic")
	if !strings.Contains(got, "Tibbo Basic") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
