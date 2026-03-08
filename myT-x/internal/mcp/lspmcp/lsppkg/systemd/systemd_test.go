package systemd

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
			name:    "direct systemd language server command",
			command: "systemd-language-server",
			want:    true,
		},
		{
			name:    "systemd lsp executable path",
			command: `C:\tools\systemd\systemd-lsp.exe`,
			want:    true,
		},
		{
			name:    "python launch with systemd language server module",
			command: "python3",
			args:    []string{"-m", "systemd_language_server"},
			want:    true,
		},
		{
			name:    "uvx launch with systemd language server package",
			command: "uvx",
			args:    []string{"systemd-language-server"},
			want:    true,
		},
		{
			name:    "wrapper command with systemd language server arg",
			command: "wrapper",
			args:    []string{"/opt/systemd-language-server/bin/systemd-language-server"},
			want:    true,
		},
		{
			name:    "python launch with unrelated module",
			command: "python3",
			args:    []string{"-m", "http.server"},
			want:    false,
		},
		{
			name:    "systemctl should not match",
			command: "systemctl",
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
	got := describeCapabilityCommand("systemd.reloadUnits", "systemd")
	if !strings.Contains(got, "systemd") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
