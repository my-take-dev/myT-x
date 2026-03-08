package ruby

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
			name:    "direct solargraph command",
			command: "solargraph",
			args:    []string{"stdio"},
			want:    true,
		},
		{
			name:    "direct language_server-ruby command",
			command: "language_server-ruby",
			want:    true,
		},
		{
			name:    "direct orbacle command",
			command: "orbacle",
			want:    true,
		},
		{
			name:    "direct ruby language server command",
			command: "ruby_language_server",
			want:    true,
		},
		{
			name:    "direct ruby lsp command",
			command: "ruby-lsp",
			want:    true,
		},
		{
			name:    "sorbet command with lsp mode",
			command: "sorbet",
			args:    []string{"--lsp"},
			want:    true,
		},
		{
			name:    "srb tc with lsp flag",
			command: "srb",
			args:    []string{"tc", "--lsp"},
			want:    true,
		},
		{
			name:    "ruby runtime with script switch and ruby lsp",
			command: "ruby",
			args:    []string{"-S", "ruby-lsp"},
			want:    true,
		},
		{
			name:    "bundle exec ruby lsp",
			command: "bundle",
			args:    []string{"exec", "ruby-lsp"},
			want:    true,
		},
		{
			name:    "arg contains solargraph path",
			command: "wrapper",
			args:    []string{`/opt/solargraph/bin/solargraph`},
			want:    true,
		},
		{
			name:    "sorbet without lsp mode does not match",
			command: "sorbet",
			args:    []string{"--version"},
			want:    false,
		},
		{
			name:    "srb typecheck without lsp mode does not match",
			command: "srb",
			args:    []string{"tc"},
			want:    false,
		},
		{
			name:    "ruby runtime with regular script",
			command: "ruby",
			args:    []string{"script.rb"},
			want:    false,
		},
		{
			name:    "bundle exec non language server command",
			command: "bundle",
			args:    []string{"exec", "rubocop"},
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
	got := describeCapabilityCommand("ruby.organizeImports", "Ruby")
	if !strings.Contains(got, "Ruby") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
