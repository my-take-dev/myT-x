package scala

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
			name:    "direct metals command",
			command: "metals",
			want:    true,
		},
		{
			name:    "scala langserver executable path",
			command: `C:\tools\scala\scala-langserver.exe`,
			want:    true,
		},
		{
			name:    "wrapper args include metals executable",
			command: "wrapper",
			args:    []string{`C:\tools\metals\metals.exe`},
			want:    true,
		},
		{
			name:    "java launch metals main",
			command: "java",
			args:    []string{"-cp", "metals.jar", "scala.meta.metals.Main"},
			want:    true,
		},
		{
			name:    "coursier launch metals",
			command: "cs",
			args:    []string{"launch", "metals"},
			want:    true,
		},
		{
			name:    "java launch unrelated class",
			command: "java",
			args:    []string{"-cp", "app.jar", "com.example.Main"},
			want:    false,
		},
		{
			name:    "unrelated command",
			command: "rust-analyzer",
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
	got := describeCapabilityCommand("metals.doctor-run", "Scala")
	if !strings.Contains(got, "Scala") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
