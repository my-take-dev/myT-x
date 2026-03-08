package stimulus

import "testing"

func TestMatches(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{
			name:    "direct stimulus language server command",
			command: "stimulus-language-server",
			want:    true,
		},
		{
			name:    "stimulus executable path",
			command: `C:\tools\stimulus-lsp\stimulus-language-server.exe`,
			want:    true,
		},
		{
			name:    "node launch with stimulus server arg",
			command: "node",
			args:    []string{`C:\tools\stimulus-lsp\out\stimulus-language-server.js`},
			want:    true,
		},
		{
			name:    "arg contains stimulus lsp path",
			command: "wrapper",
			args:    []string{`C:\tools\stimulus-lsp\bin\stimulus-lsp`},
			want:    true,
		},
		{
			name:    "non stimulus command",
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
