package wolfram

import "testing"

func TestMatches(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{
			name:    "direct lsp wl command",
			command: "lsp-wl",
			want:    true,
		},
		{
			name:    "wlsp executable path",
			command: `C:\tools\wolfram\wlsp.exe`,
			want:    true,
		},
		{
			name:    "wolframscript launch with LSPServer argument",
			command: "wolframscript",
			args:    []string{"-file", `C:\Wolfram\Applications\LSPServer\Kernel\LSPServer.wl`},
			want:    true,
		},
		{
			name:    "wrapper args include wolfram language server",
			command: "wrapper",
			args:    []string{`/opt/wolfram-language-server/bin/wolfram-language-server`},
			want:    true,
		},
		{
			name:    "wolfram cli should not match",
			command: "wolfram",
			want:    false,
		},
		{
			name:    "unrelated command should not match",
			command: "python",
			args:    []string{"-m", "pylsp"},
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
