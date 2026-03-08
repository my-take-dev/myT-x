package rego

import "testing"

func TestMatches(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{
			name:    "direct regal language server command",
			command: "regal-language-server",
			want:    true,
		},
		{
			name:    "regal cli language-server mode",
			command: "regal",
			args:    []string{"language-server"},
			want:    true,
		},
		{
			name:    "regal cli lsp mode",
			command: "regal.exe",
			args:    []string{"lsp"},
			want:    true,
		},
		{
			name:    "go run regal language server",
			command: "go",
			args:    []string{"run", "github.com/StyraInc/regal", "language-server"},
			want:    true,
		},
		{
			name:    "docker image launch",
			command: "docker",
			args:    []string{"run", "--rm", "ghcr.io/styrainc/regal:latest", "language-server"},
			want:    true,
		},
		{
			name:    "regal non lsp mode",
			command: "regal",
			args:    []string{"fmt", "./policy"},
			want:    false,
		},
		{
			name:    "go run regal without lsp mode",
			command: "go",
			args:    []string{"run", "github.com/StyraInc/regal", "version"},
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
