package perl

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
			name:    "direct perl language server command",
			command: "perl-languageserver",
			want:    true,
		},
		{
			name:    "perl runtime with perl language server module",
			command: "perl",
			args:    []string{"-MPerl::LanguageServer", "-e", "Perl::LanguageServer::run"},
			want:    true,
		},
		{
			name:    "direct pls command",
			command: "pls",
			want:    true,
		},
		{
			name:    "pls executable path",
			command: `C:\tools\perlnavigator\pls\pls.exe`,
			want:    true,
		},
		{
			name:    "direct perl navigator command",
			command: "perlnavigator",
			want:    true,
		},
		{
			name:    "node launch with perl navigator arg",
			command: "node",
			args:    []string{`C:\tools\PerlNavigator\server\out\server.js`},
			want:    true,
		},
		{
			name:    "arg contains perl navigator executable path",
			command: "wrapper",
			args:    []string{`/opt/perlnavigator/bin/perlnavigator`},
			want:    true,
		},
		{
			name:    "perl runtime with regular script",
			command: "perl",
			args:    []string{"script.pl"},
			want:    false,
		},
		{
			name:    "node launch with unrelated script",
			command: "node",
			args:    []string{`C:\tools\scripts\main.js`},
			want:    false,
		},
		{
			name:    "unrelated command with pls substring",
			command: "wrapper",
			args:    []string{`C:\tools\please.exe`},
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
	got := describeCapabilityCommand("perl.organizeImports", "Perl")
	if !strings.Contains(got, "Perl") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
