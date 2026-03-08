package sparql

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
			name:    "direct qlue ls command",
			command: "qlue-ls",
			want:    true,
		},
		{
			name:    "direct sparql language server command",
			command: "sparql-language-server",
			want:    true,
		},
		{
			name:    "node launch with qlue ls script",
			command: "node",
			args:    []string{`/opt/qlue-ls/dist/server.js`},
			want:    true,
		},
		{
			name:    "java launch with stardog sparql language server jar",
			command: "java",
			args:    []string{"-jar", `C:\tools\sparql-language-server\sparql-language-server.jar`},
			want:    true,
		},
		{
			name:    "arg contains sparql language server path",
			command: "wrapper",
			args:    []string{`/usr/local/bin/sparql-language-server`},
			want:    true,
		},
		{
			name:    "non sparql command",
			command: "graphql-language-service-cli",
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
	got := describeCapabilityCommand("sparql.executeQueryPlan", "SPARQL")
	if !strings.Contains(got, "SPARQL") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
