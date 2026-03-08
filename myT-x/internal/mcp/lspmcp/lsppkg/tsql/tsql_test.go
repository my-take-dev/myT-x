package tsql

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
			name:    "direct sqltoolsservice command",
			command: "sqltoolsservice",
			want:    true,
		},
		{
			name:    "microsoft sqltools service layer executable path",
			command: `C:\tools\mssql\Microsoft.SqlTools.ServiceLayer.exe`,
			want:    true,
		},
		{
			name:    "node launch with vscode mssql extension path",
			command: "node",
			args:    []string{"/extensions/vscode-mssql/out/src/languageService/main.js"},
			want:    true,
		},
		{
			name:    "dotnet launch with sql tools service layer dll",
			command: "dotnet",
			args:    []string{"/opt/vscode-mssql/sqltoolsservice/Microsoft.SqlTools.ServiceLayer.dll"},
			want:    true,
		},
		{
			name:    "wrapper command with mssql language server arg",
			command: "wrapper",
			args:    []string{"/usr/local/bin/mssql-language-server"},
			want:    true,
		},
		{
			name:    "node launch with unrelated script",
			command: "node",
			args:    []string{"/opt/tools/index.js"},
			want:    false,
		},
		{
			name:    "sqlcmd should not match",
			command: "sqlcmd",
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
	got := describeCapabilityCommand("tsql.workspace.refresh", "T-SQL")
	if !strings.Contains(got, "T-SQL") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
