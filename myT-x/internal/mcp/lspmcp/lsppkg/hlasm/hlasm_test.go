package hlasm

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
			name:    "direct hlasm language server command",
			command: "hlasm-language-server",
			want:    true,
		},
		{
			name:    "hlasm executable path",
			command: `C:\tools\hlasm-language-server\hlasm-language-server.exe`,
			want:    true,
		},
		{
			name:    "java launch with zopeneditor arg",
			command: "java",
			args:    []string{"-jar", `C:\tools\zopeneditor\hlasm-language-server.jar`},
			want:    true,
		},
		{
			name:    "arg contains hlasm path",
			command: "wrapper",
			args:    []string{`C:\tools\hlasm-language-server\hlasm-language-server.cmd`},
			want:    true,
		},
		{
			name:    "non hlasm command",
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
	got := describeCapabilityCommand("hlasm.build", "IBM High Level Assembler")
	if !strings.Contains(got, "IBM High Level Assembler") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
