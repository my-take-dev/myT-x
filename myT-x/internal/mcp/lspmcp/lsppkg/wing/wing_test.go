package wing

import "testing"

func TestMatches(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{
			name:    "direct wing language server command",
			command: "wing-language-server",
			want:    true,
		},
		{
			name:    "wingls executable path",
			command: `C:\tools\wing\wingls.exe`,
			want:    true,
		},
		{
			name:    "wing cli lsp mode",
			command: "wing",
			args:    []string{"lsp", "--stdio"},
			want:    true,
		},
		{
			name:    "wrapper args include wing lsp path",
			command: "wrapper",
			args:    []string{`/opt/wing-lsp/bin/wing-lsp`},
			want:    true,
		},
		{
			name:    "wing compile command should not match",
			command: "wing",
			args:    []string{"compile"},
			want:    false,
		},
		{
			name:    "unrelated command should not match",
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
