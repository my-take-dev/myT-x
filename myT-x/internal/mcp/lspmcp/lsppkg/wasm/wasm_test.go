package wasm

import "testing"

func TestMatches(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{
			name:    "direct wasm language server command",
			command: "wasm-language-server",
			want:    true,
		},
		{
			name:    "wasm lsp executable path",
			command: `C:\tools\wasm\wasm-lsp.exe`,
			want:    true,
		},
		{
			name:    "node launch with wasm language tools reference",
			command: "node",
			args:    []string{`C:\repo\node_modules\wasm-language-tools\bin\server.js`},
			want:    true,
		},
		{
			name:    "wrapper args include wasm language server path",
			command: "wrapper",
			args:    []string{`/opt/wasm-language-server/bin/wasm-language-server`},
			want:    true,
		},
		{
			name:    "wasmtime should not match",
			command: "wasmtime",
			want:    false,
		},
		{
			name:    "unrelated server should not match",
			command: "rust-analyzer",
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
