package vdm

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
			name:    "direct vdmj lsp command",
			command: "vdmj-lsp",
			want:    true,
		},
		{
			name:    "java launch with vdmj lsp jar",
			command: "java",
			args:    []string{"-jar", "/opt/vdmj/vdmj-lsp.jar"},
			want:    true,
		},
		{
			name:    "wrapper args contain vdmpp lsp binary",
			command: "wrapper",
			args:    []string{`C:\tools\vdm\vdmpp-lsp.exe`},
			want:    true,
		},
		{
			name:    "java without vdm lsp reference",
			command: "java",
			args:    []string{"-version"},
			want:    false,
		},
		{
			name:    "vdmj command without lsp should not match",
			command: "vdmj",
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
	got := describeCapabilityCommand("workspace/executeCommand", "VDM")
	if !strings.Contains(got, "VDM") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
