package dart

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
			name:    "direct dart language server command",
			command: "dart-language-server",
			want:    true,
		},
		{
			name:    "analysis server snapshot path",
			command: `C:\tools\dart-sdk\bin\snapshots\analysis_server.dart.snapshot`,
			want:    true,
		},
		{
			name:    "dart launcher with language server args",
			command: "dart",
			args:    []string{"language-server", "--protocol=lsp"},
			want:    true,
		},
		{
			name:    "arg contains analysis server snapshot",
			command: "wrapper",
			args:    []string{`C:\tools\dart-sdk\bin\snapshots\analysis_server.dart.snapshot`},
			want:    true,
		},
		{
			name:    "non dart command",
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
	got := describeCapabilityCommand("dart.run", "Dart")
	if !strings.Contains(got, "Dart") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
