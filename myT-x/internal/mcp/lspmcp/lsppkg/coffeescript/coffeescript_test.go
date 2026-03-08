package coffeescript

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
			name:    "direct coffeesense command",
			command: "coffeesense",
			want:    true,
		},
		{
			name:    "coffeesense executable path",
			command: `C:\tools\coffeesense\coffeesense.cmd`,
			want:    true,
		},
		{
			name:    "node launch with coffeesense arg",
			command: "node",
			args:    []string{`C:\tools\coffeesense\dist\server.js`},
			want:    true,
		},
		{
			name:    "arg contains coffeesense path",
			command: "wrapper",
			args:    []string{`C:\tools\coffeesense\coffeesense`},
			want:    true,
		},
		{
			name:    "non coffeescript command",
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
	got := describeCapabilityCommand("coffeescript.compile", "CoffeeScript")
	if !strings.Contains(got, "CoffeeScript") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
