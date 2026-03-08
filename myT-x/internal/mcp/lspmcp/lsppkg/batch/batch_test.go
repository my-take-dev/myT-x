package batch

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
			name:    "direct batch language server command",
			command: "batch-language-server",
			want:    true,
		},
		{
			name:    "batch executable path",
			command: `C:\tools\rech-editor-batch\rech-editor-batch.exe`,
			want:    true,
		},
		{
			name:    "node launch with rech editor batch arg",
			command: "node",
			args:    []string{`C:\tools\rech-editor-batch\dist\server.js`},
			want:    true,
		},
		{
			name:    "arg contains batch language server path",
			command: "wrapper",
			args:    []string{`C:\tools\batch-language-server\batch-language-server.cmd`},
			want:    true,
		},
		{
			name:    "non batch command",
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
	got := describeCapabilityCommand("batch.run", "Batch")
	if !strings.Contains(got, "Batch") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
