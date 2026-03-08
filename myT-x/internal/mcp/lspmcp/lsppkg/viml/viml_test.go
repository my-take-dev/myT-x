package viml

import "testing"

func TestMatches(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{
			name:    "direct vim language server command",
			command: "vim-language-server",
			want:    true,
		},
		{
			name:    "vimls executable path",
			command: `C:\tools\vim\vimls.exe`,
			want:    true,
		},
		{
			name:    "node launch with vim language server arg",
			command: "node",
			args:    []string{`C:\tools\vim-language-server\bin\index.js`},
			want:    true,
		},
		{
			name:    "args include vim language server under wrapper",
			command: "wrapper",
			args:    []string{`/opt/vim-language-server/bin/vim-language-server`},
			want:    true,
		},
		{
			name:    "similarly named vim binary should not match",
			command: "vim",
			want:    false,
		},
		{
			name:    "unrelated command",
			command: "lua-language-server",
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
