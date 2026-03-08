package ocamlreason

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
			name:    "direct ocamllsp command",
			command: "ocamllsp",
			want:    true,
		},
		{
			name:    "ocaml lsp server executable path",
			command: `C:\tools\ocaml-lsp\ocaml-lsp-server.exe`,
			want:    true,
		},
		{
			name:    "opam launch with ocamllsp",
			command: "opam",
			args:    []string{"exec", "--", "ocamllsp", "--stdio"},
			want:    true,
		},
		{
			name:    "esy launch with ocamllsp path",
			command: "esy",
			args:    []string{"x", `/opt/ocaml-lsp/bin/ocamllsp`},
			want:    true,
		},
		{
			name:    "dune launch with reason language server",
			command: "dune",
			args:    []string{"exec", "--", "reason-language-server"},
			want:    true,
		},
		{
			name:    "opam launch without ocaml lsp reference",
			command: "opam",
			args:    []string{"exec", "--", "utop"},
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
	got := describeCapabilityCommand("ocamlreason.switchImplIntf", "OCaml/Reason")
	if !strings.Contains(got, "OCaml/Reason") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
