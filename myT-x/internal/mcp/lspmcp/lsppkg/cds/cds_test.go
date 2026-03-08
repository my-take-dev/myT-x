package cds

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
			name:    "direct cds lsp command",
			command: "cds-lsp",
			want:    true,
		},
		{
			name:    "cds executable path",
			command: `C:\tools\cds-lsp\cds-lsp.cmd`,
			want:    true,
		},
		{
			name:    "node launch with cds arg",
			command: "node",
			args:    []string{`C:\tools\@sap\cds-lsp\dist\server.js`},
			want:    true,
		},
		{
			name:    "arg contains cds lsp path",
			command: "wrapper",
			args:    []string{`C:\tools\cds-lsp\cds-lsp`},
			want:    true,
		},
		{
			name:    "non cds command",
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
	got := describeCapabilityCommand("cds.compile", "CDS")
	if !strings.Contains(got, "CDS") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
