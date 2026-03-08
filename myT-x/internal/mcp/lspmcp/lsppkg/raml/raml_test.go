package raml

import "testing"

func TestMatches(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{
			name:    "direct raml language server command",
			command: "raml-language-server",
			want:    true,
		},
		{
			name:    "raml language server executable path",
			command: `C:\tools\raml-language-server\raml-language-server.cmd`,
			want:    true,
		},
		{
			name:    "node launch with raml language server script",
			command: "node",
			args:    []string{"./node_modules/raml-language-server/dist/server.js", "--stdio"},
			want:    true,
		},
		{
			name:    "npx launch",
			command: "npx",
			args:    []string{"-y", "raml-language-server", "--stdio"},
			want:    true,
		},
		{
			name:    "npm exec launch",
			command: "npm",
			args:    []string{"exec", "raml-language-server", "--", "--stdio"},
			want:    true,
		},
		{
			name:    "node without raml server reference",
			command: "node",
			args:    []string{"./scripts/build.js"},
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
