package asn1

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
			name:    "direct titan language server command",
			command: "titan-language-server",
			want:    true,
		},
		{
			name:    "asn1 executable path",
			command: `C:\tools\asn1-language-server\asn1-language-server.exe`,
			want:    true,
		},
		{
			name:    "java launch with titan core arg",
			command: "java",
			args:    []string{"-jar", `C:\tools\titan.core\titan-language-server.jar`},
			want:    true,
		},
		{
			name:    "arg contains titan language server path",
			command: "wrapper",
			args:    []string{`C:\tools\titan-language-server\titan-language-server.cmd`},
			want:    true,
		},
		{
			name:    "non asn1 command",
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
	got := describeCapabilityCommand("asn1.validate", "ASN.1")
	if !strings.Contains(got, "ASN.1") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
