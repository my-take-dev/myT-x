package kotlin

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
			name:    "direct kotlin language server command",
			command: "kotlin-language-server",
			want:    true,
		},
		{
			name:    "direct kotlin lsp command",
			command: "kotlin-lsp",
			want:    true,
		},
		{
			name:    "kotlin language server executable path",
			command: `C:\tools\kotlin\kotlin-language-server.exe`,
			want:    true,
		},
		{
			name:    "java launch with kotlin language server jar",
			command: "java",
			args:    []string{"-jar", `/opt/kotlin-language-server/server.jar`},
			want:    true,
		},
		{
			name:    "arg contains kotlin lsp path",
			command: "wrapper",
			args:    []string{`/opt/jetbrains/kotlin-lsp/bin/kotlin-lsp`},
			want:    true,
		},
		{
			name:    "java launch with unrelated jar should not match",
			command: "java",
			args:    []string{"-jar", `/opt/tools/checkstyle.jar`},
			want:    false,
		},
		{
			name:    "non kotlin command",
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
	got := describeCapabilityCommand("kotlin.organizeImports", "Kotlin")
	if !strings.Contains(got, "Kotlin") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
