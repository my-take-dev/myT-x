package raku

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
			name:    "direct raku-navigator command",
			command: "raku-navigator",
			want:    true,
		},
		{
			name:    "rakunavigator executable path",
			command: `C:\tools\RakuNavigator\rakunavigator.exe`,
			want:    true,
		},
		{
			name:    "node launch with raku navigator server script",
			command: "node",
			args:    []string{`C:\tools\RakuNavigator\server\out\server.js`},
			want:    true,
		},
		{
			name:    "npx launch with raku navigator package",
			command: "npx",
			args:    []string{"@bscan/raku-navigator", "--stdio"},
			want:    true,
		},
		{
			name:    "arg contains raku navigator path",
			command: "wrapper",
			args:    []string{`/opt/rakunavigator/bin/raku-navigator`},
			want:    true,
		},
		{
			name:    "node launch with unrelated script",
			command: "node",
			args:    []string{`C:\tools\server\out\server.js`},
			want:    false,
		},
		{
			name:    "raku runtime should not match",
			command: "raku",
			args:    []string{"script.raku"},
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
	got := describeCapabilityCommand("raku.formatDocument", "Raku")
	if !strings.Contains(got, "Raku") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
