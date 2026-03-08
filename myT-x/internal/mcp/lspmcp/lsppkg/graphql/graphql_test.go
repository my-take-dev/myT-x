package graphql

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
			name:    "direct graphql lsp command",
			command: "graphql-lsp",
			want:    true,
		},
		{
			name:    "graphql language service server executable path",
			command: `C:\tools\graphql-language-service-server\graphql-language-service-server.exe`,
			want:    true,
		},
		{
			name:    "node launch with official graphql language service package",
			command: "node",
			args:    []string{`C:\repo\node_modules\@graphql-language-service\server\dist\server.js`, "--stdio"},
			want:    true,
		},
		{
			name:    "npx launch with gql language server package",
			command: "npx",
			args:    []string{"gql-language-server", "--stdio"},
			want:    true,
		},
		{
			name:    "arg contains gql language server executable path",
			command: "wrapper",
			args:    []string{`C:\tools\gql-language-server\bin\gql-language-server.exe`},
			want:    true,
		},
		{
			name:    "non graphql command",
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
	got := describeCapabilityCommand("graphql.cacheSchema", "GraphQL")
	if !strings.Contains(got, "GraphQL") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
