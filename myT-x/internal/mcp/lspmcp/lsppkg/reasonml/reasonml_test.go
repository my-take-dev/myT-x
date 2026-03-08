package reasonml

import "testing"

func TestMatches(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{
			name:    "direct reason language server command",
			command: "reason-language-server",
			want:    true,
		},
		{
			name:    "reason language server executable path",
			command: `C:\tools\reason-language-server\reason-language-server.cmd`,
			want:    true,
		},
		{
			name:    "node launch with reason language server script",
			command: "node",
			args:    []string{"./node_modules/reason-language-server/bin/server.js", "--stdio"},
			want:    true,
		},
		{
			name:    "npx launch",
			command: "npx",
			args:    []string{"reason-language-server", "--stdio"},
			want:    true,
		},
		{
			name:    "opam launch",
			command: "opam",
			args:    []string{"exec", "--", "reason-language-server", "--stdio"},
			want:    true,
		},
		{
			name:    "esy launch",
			command: "esy",
			args:    []string{"x", "reason-language-server", "--stdio"},
			want:    true,
		},
		{
			name:    "opam without reason language server reference",
			command: "opam",
			args:    []string{"exec", "--", "utop"},
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
