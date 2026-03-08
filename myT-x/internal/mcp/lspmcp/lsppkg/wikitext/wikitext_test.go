package wikitext

import "testing"

func TestMatches(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{
			name:    "direct wikitext language server command",
			command: "wikitext-language-server",
			want:    true,
		},
		{
			name:    "wikitext lsp executable path",
			command: `C:\tools\wikitext\wikitext-lsp.exe`,
			want:    true,
		},
		{
			name:    "node launch with mediawiki language server arg",
			command: "node",
			args:    []string{`C:\repo\node_modules\mediawiki-language-server\dist\server.js`},
			want:    true,
		},
		{
			name:    "wrapper args include wikitext server path",
			command: "wrapper",
			args:    []string{`/opt/wikitext-language-server/bin/wikitext-language-server`},
			want:    true,
		},
		{
			name:    "wiki cli should not match",
			command: "wikicli",
			want:    false,
		},
		{
			name:    "unrelated markdown server should not match",
			command: "marksman",
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
