package java

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
			name:    "direct jdtls command",
			command: "jdtls",
			want:    true,
		},
		{
			name:    "jdtls executable path",
			command: `C:\tools\jdtls\bin\jdtls.cmd`,
			want:    true,
		},
		{
			name:    "java launch with jdtls equinox launcher",
			command: "java",
			args: []string{
				"-jar",
				`C:\tools\jdtls\plugins\org.eclipse.equinox.launcher_1.6.900.v20240613-2009.jar`,
				"-configuration",
				`C:\tools\jdtls\config_win`,
				"-data",
				`C:\workspace`,
				`-Dorg.eclipse.jdt.ls.log.level=ALL`,
			},
			want: true,
		},
		{
			name:    "direct java-language-server command",
			command: "java-language-server",
			want:    true,
		},
		{
			name:    "arg contains java-language-server path",
			command: "wrapper",
			args:    []string{`/usr/local/bin/java-language-server`},
			want:    true,
		},
		{
			name:    "java launch with java-language-server jar",
			command: "java",
			args:    []string{"-jar", `/opt/java-language-server/java-language-server.jar`},
			want:    true,
		},
		{
			name:    "java launch with unrelated jar should not match",
			command: "java",
			args:    []string{"-jar", `/opt/tools/checkstyle.jar`},
			want:    false,
		},
		{
			name:    "non java command",
			command: "javac",
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
	got := describeCapabilityCommand("java.organizeImports", "Java")
	if !strings.Contains(got, "Java") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
