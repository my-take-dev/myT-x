package rlang

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
			name:    "direct languageserver command",
			command: "languageserver",
			want:    true,
		},
		{
			name:    "r-languageserver executable path",
			command: `C:\tools\r-languageserver\r-languageserver.cmd`,
			want:    true,
		},
		{
			name:    "r runtime with languageserver run expression",
			command: "R",
			args:    []string{"--no-echo", "-e", "languageserver::run()"},
			want:    true,
		},
		{
			name:    "rscript with languageserver run expression",
			command: "Rscript",
			args:    []string{"-e", "languageserver::run()"},
			want:    true,
		},
		{
			name:    "arg contains languageserver path",
			command: "wrapper",
			args:    []string{`/opt/languageserver/bin/languageserver`},
			want:    true,
		},
		{
			name:    "rscript with regular script",
			command: "Rscript",
			args:    []string{"analysis.R"},
			want:    false,
		},
		{
			name:    "different language server should not match",
			command: "languagetool-languageserver",
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
	got := describeCapabilityCommand("r.executeSelection", "R")
	if !strings.Contains(got, "R") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
