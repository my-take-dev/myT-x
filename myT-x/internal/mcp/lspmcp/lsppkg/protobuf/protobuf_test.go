package protobuf

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
			name:    "direct protols command",
			command: "protols",
			want:    true,
		},
		{
			name:    "direct protobuf language server command",
			command: "protobuf-language-server",
			want:    true,
		},
		{
			name:    "direct bufls command",
			command: "bufls",
			want:    true,
		},
		{
			name:    "go run with protobuf language server module path",
			command: "go",
			args:    []string{"run", "github.com/lasorda/protobuf-language-server/cmd/protobuf-language-server@latest"},
			want:    true,
		},
		{
			name:    "cargo run protols",
			command: "cargo",
			args:    []string{"run", "--bin", "protols"},
			want:    true,
		},
		{
			name:    "buf lsp serve subcommand",
			command: "buf",
			args:    []string{"lsp", "serve", "--stdio"},
			want:    true,
		},
		{
			name:    "wrapper arg contains protobuf language server path",
			command: "wrapper",
			args:    []string{`C:\tools\protobuf-language-server\protobuf-language-server.exe`},
			want:    true,
		},
		{
			name:    "buf build command should not match",
			command: "buf",
			args:    []string{"build"},
			want:    false,
		},
		{
			name:    "go run protoc gen plugin should not match",
			command: "go",
			args:    []string{"run", "google.golang.org/protobuf/cmd/protoc-gen-go@latest"},
			want:    false,
		},
		{
			name:    "cargo run unrelated binary should not match",
			command: "cargo",
			args:    []string{"run", "--bin", "rust-analyzer"},
			want:    false,
		},
		{
			name:    "protoc command should not match",
			command: "protoc",
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
	got := describeCapabilityCommand("protobuf.generateDescriptorSet", "Protocol Buffers")
	if !strings.Contains(got, "Protocol Buffers") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
