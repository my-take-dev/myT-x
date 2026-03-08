package xml

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
			name:    "direct xml language server command",
			command: "xml-language-server",
			want:    true,
		},
		{
			name:    "direct lemminx command",
			command: "lemminx",
			want:    true,
		},
		{
			name:    "node launch with xml language server script",
			command: "node",
			args:    []string{`C:\tools\xml-language-server\lib\server.js`},
			want:    true,
		},
		{
			name:    "java launch with lemminx jar",
			command: "java",
			args:    []string{"-jar", `C:\tools\lemminx\org.eclipse.lemminx-uber.jar`},
			want:    true,
		},
		{
			name:    "wrapper args contain lemminx repo hint",
			command: "wrapper",
			args:    []string{"--source", "eclipse/lemminx"},
			want:    true,
		},
		{
			name:    "java launch without xml reference",
			command: "java",
			args:    []string{"-jar", `C:\tools\jdtls\plugins\org.eclipse.equinox.launcher.jar`},
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
	got := describeCapabilityCommand("xml.updateCatalog", "XML")
	if !strings.Contains(got, "XML") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
