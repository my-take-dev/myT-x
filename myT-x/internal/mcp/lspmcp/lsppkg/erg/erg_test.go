package erg

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
			name:    "direct els command",
			command: "els",
			want:    true,
		},
		{
			name:    "erg language server executable path",
			command: `C:\tools\erg\erg-language-server.exe`,
			want:    true,
		},
		{
			name:    "erg runner with language server flag",
			command: "erg",
			args:    []string{"--language-server"},
			want:    true,
		},
		{
			name:    "arg contains els path",
			command: "wrapper",
			args:    []string{`C:\tools\erg\bin\els.exe`},
			want:    true,
		},
		{
			name:    "erg compile command should not match",
			command: "erg",
			args:    []string{"check", "main.er"},
			want:    false,
		},
		{
			name:    "non erg command",
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
	got := describeCapabilityCommand("erg.runCodeAction", "Erg")
	if !strings.Contains(got, "Erg") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
