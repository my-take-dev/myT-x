package elixir

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
			name:    "direct elixir-ls command",
			command: "elixir-ls",
			want:    true,
		},
		{
			name:    "language server script path",
			command: `C:\tools\elixir-ls\language_server.bat`,
			want:    true,
		},
		{
			name:    "elixir runner with language server script",
			command: "elixir",
			args:    []string{`C:\tools\elixir-ls\language_server.sh`},
			want:    true,
		},
		{
			name:    "mix runner with language server entrypoint",
			command: "mix",
			args:    []string{"run", "--no-halt", "language_server.exs"},
			want:    true,
		},
		{
			name:    "elixir command without lsp should not match",
			command: "elixir",
			args:    []string{"-e", "IO.puts(\"hello\")"},
			want:    false,
		},
		{
			name:    "non elixir command",
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
	got := describeCapabilityCommand("elixir.expandMacro", "Elixir")
	if !strings.Contains(got, "Elixir") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
