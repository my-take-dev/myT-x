package qmlls

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
			name:    "direct qmlls command",
			command: "qmlls",
			want:    true,
		},
		{
			name:    "absolute qmlls.exe path",
			command: `C:\Qt\bin\qmlls.exe`,
			want:    true,
		},
		{
			name:    "arg contains qmlls path",
			command: "wrapper",
			args:    []string{`C:\Qt\bin\qmlls.exe`},
			want:    true,
		},
		{
			name:    "non qmlls command",
			command: "clangd",
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
	got := describeCapabilityCommand("qmlls.applyFix", "QML")
	if !strings.Contains(got, "QML") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
