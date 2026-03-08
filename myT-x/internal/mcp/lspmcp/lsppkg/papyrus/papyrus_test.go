package papyrus

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
			name:    "direct papyrus lang command",
			command: "papyrus-lang",
			want:    true,
		},
		{
			name:    "papyrus lang executable path",
			command: `C:\tools\papyrus-lang\papyrus-lang.exe`,
			want:    true,
		},
		{
			name:    "dotnet launch with papyrus lang dll",
			command: "dotnet",
			args:    []string{`C:\tools\papyrus-lang\papyrus-lang.dll`},
			want:    true,
		},
		{
			name:    "arg contains papyrus lang path",
			command: "wrapper",
			args:    []string{`C:\src\joelday\papyrus-lang\bin\papyrus-lang`},
			want:    true,
		},
		{
			name:    "dotnet launch with unrelated dll",
			command: "dotnet",
			args:    []string{`C:\tools\other\server.dll`},
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
	got := describeCapabilityCommand("papyrus.organizeImports", "Papyrus")
	if !strings.Contains(got, "Papyrus") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
