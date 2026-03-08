package rascal

import "testing"

func TestMatches(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{
			name:    "direct rascal language server command",
			command: "rascal-language-server",
			want:    true,
		},
		{
			name:    "rascal lsp executable path",
			command: `C:\tools\rascal\rascal-lsp.exe`,
			want:    true,
		},
		{
			name:    "java jar launch",
			command: "java",
			args:    []string{"-jar", "/opt/rascal-language-server.jar", "--stdio"},
			want:    true,
		},
		{
			name:    "java classpath launch",
			command: "java",
			args:    []string{"-cp", "/opt/rascal-lsp.jar", "org.rascalmpl.vscode.lsp.Main"},
			want:    true,
		},
		{
			name:    "npx launch",
			command: "npx",
			args:    []string{"rascal-language-server", "--stdio"},
			want:    true,
		},
		{
			name:    "java without rascal reference",
			command: "java",
			args:    []string{"-jar", "/opt/some-other-server.jar"},
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
