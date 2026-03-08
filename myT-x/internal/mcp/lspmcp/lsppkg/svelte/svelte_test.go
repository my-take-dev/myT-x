package svelte

import "testing"

func TestMatches(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{
			name:    "direct svelteserver command",
			command: "svelteserver",
			want:    true,
		},
		{
			name:    "svelte language server executable path",
			command: `C:\tools\svelte-language-server\svelte-language-server.exe`,
			want:    true,
		},
		{
			name:    "node launch with svelte language server arg",
			command: "node",
			args:    []string{`C:\tools\svelte-language-server\bin\server.js`},
			want:    true,
		},
		{
			name:    "arg contains svelteserver path",
			command: "wrapper",
			args:    []string{`C:\tools\svelte-language-server\bin\svelteserver`},
			want:    true,
		},
		{
			name:    "non svelte command",
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
