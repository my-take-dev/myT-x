package vue

import "testing"

func TestMatches(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{
			name:    "direct vls command",
			command: "vls",
			want:    true,
		},
		{
			name:    "vue language server executable path",
			command: `C:\tools\vue\vue-language-server.exe`,
			want:    true,
		},
		{
			name:    "node launch with vue language tools package arg",
			command: "node",
			args:    []string{`C:\repo\node_modules\@vue\language-server\bin\vue-language-server.js`},
			want:    true,
		},
		{
			name:    "npx launch with vetur language server reference",
			command: "npx",
			args:    []string{"vetur-language-server", "--stdio"},
			want:    true,
		},
		{
			name:    "similarly named vue cli should not match",
			command: "vue",
			want:    false,
		},
		{
			name:    "unrelated language server",
			command: "typescript-language-server",
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
