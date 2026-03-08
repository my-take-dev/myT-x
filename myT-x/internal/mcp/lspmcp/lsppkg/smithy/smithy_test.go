package smithy

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
			name:    "direct smithy language server command",
			command: "smithy-language-server",
			want:    true,
		},
		{
			name:    "smithy language server executable path",
			command: `C:\tools\smithy-language-server\smithy-language-server.exe`,
			want:    true,
		},
		{
			name:    "java launch with smithy language server jar",
			command: "java",
			args:    []string{"-jar", `C:\tools\smithy-language-server\smithy-language-server.jar`},
			want:    true,
		},
		{
			name:    "arg contains smithy language server path",
			command: "wrapper",
			args:    []string{`/opt/smithy-language-server/bin/smithy-language-server`},
			want:    true,
		},
		{
			name:    "non smithy command",
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
	got := describeCapabilityCommand("smithy.applyModelTransform", "Smithy")
	if !strings.Contains(got, "Smithy") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
