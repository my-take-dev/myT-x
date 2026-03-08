package jsonnet

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
			name:    "direct jsonnet language server command",
			command: "jsonnet-language-server",
			want:    true,
		},
		{
			name:    "jsonnet language server executable path",
			command: `C:\tools\jsonnet-language-server\jsonnet-language-server.exe`,
			want:    true,
		},
		{
			name:    "go run grafana jsonnet language server module",
			command: "go",
			args:    []string{"run", "github.com/grafana/jsonnet-language-server/cmd/lsp"},
			want:    true,
		},
		{
			name:    "arg contains jsonnet language server path",
			command: "wrapper",
			args:    []string{`/opt/servers/jsonnet-language-server/bin/server`},
			want:    true,
		},
		{
			name:    "go run unrelated module should not match",
			command: "go",
			args:    []string{"run", "github.com/google/go-jsonnet/cmd/jsonnet"},
			want:    false,
		},
		{
			name:    "json language server should not match",
			command: "json-language-server",
			want:    false,
		},
		{
			name:    "non jsonnet command",
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
	got := describeCapabilityCommand("jsonnet.evaluate", "Jsonnet")
	if !strings.Contains(got, "Jsonnet") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
