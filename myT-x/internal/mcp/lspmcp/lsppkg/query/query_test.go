package query

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
			name:    "direct ts_query_ls command",
			command: "ts_query_ls",
			want:    true,
		},
		{
			name:    "ts-query-ls executable path",
			command: `C:\tools\ts-query-ls\ts-query-ls.exe`,
			want:    true,
		},
		{
			name:    "cargo launch with ts_query_ls bin",
			command: "cargo",
			args:    []string{"run", "--bin", "ts_query_ls", "--", "--stdio"},
			want:    true,
		},
		{
			name:    "arg contains ts_query_ls path",
			command: "wrapper",
			args:    []string{`/opt/ts_query_ls/bin/ts_query_ls`},
			want:    true,
		},
		{
			name:    "cargo launch with unrelated bin",
			command: "cargo",
			args:    []string{"run", "--bin", "rust-analyzer"},
			want:    false,
		},
		{
			name:    "query tool command should not match",
			command: "query",
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
	got := describeCapabilityCommand("query.execute", "Query")
	if !strings.Contains(got, "Query") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
